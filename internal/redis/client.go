// Package redis provides Redis Streams integration for Ottoscaler.
//
// 이 패키지는 Redis Streams를 이용한 스케일링 이벤트 처리를 담당합니다.
// Consumer Group 패턴을 사용하여 여러 Ottoscaler 인스턴스가 동시에
// 실행되어도 이벤트가 중복 처리되지 않도록 보장합니다.
//
// 주요 기능:
//   - Redis Streams 이벤트 수신 (ListenForScaleEvents)
//   - Consumer Group 기반 분산 처리
//   - 자동 재연결 및 에러 복구
//   - 이벤트 발행 (테스트용)
//
// 사용 예시:
//
//	client := redis.NewClient("localhost:6379", "", 0)
//	eventChan, err := client.ListenForScaleEvents(ctx, "otto:scale:events", "ottoscaler", "instance-1")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	for event := range eventChan {
//		log.Printf("Received: %s", event.Type)
//	}
package redis

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// EventChannelBuffer는 이벤트 채널의 버퍼 크기입니다
	EventChannelBuffer = 100
	// PollTimeout은 Redis 스트림 폴링 타임아웃입니다
	PollTimeout = 2 * time.Second
	// RetryDelay는 에러 발생 시 재시도 지연 시간입니다
	RetryDelay = 1 * time.Second
	// MaxMessagesPerPoll은 한 번에 읽어올 최대 메시지 수입니다
	MaxMessagesPerPoll = 10
)

// Client provides Redis Streams integration for scaling events.
//
// Client는 Redis Streams 클라이언트입니다.
// Consumer Group 패턴을 사용하여 여러 Ottoscaler 인스턴스가
// 동시에 실행되어도 이벤트가 중복 처리되지 않도록 보장합니다.
type Client struct {
	client *redis.Client
}

// ScaleEvent represents a scaling event received from Redis stream.
//
// ScaleEvent는 Redis 스트림에서 수신한 스케일링 이벤트를 나타냅니다.
//
// 이벤트 구조:
//   - EventID: Redis 스트림 메시지 ID (예: "1756659802903-0")
//   - Type: 스케일링 유형 ("scale_up" 또는 "scale_down")
//   - PodCount: 대상 Pod 수
//   - Timestamp: 이벤트 생성 시간
//   - Metadata: 작업 ID 등 추가 정보
type ScaleEvent struct {
	EventID   string            `json:"event_id"`  // Redis 메시지 ID
	Type      string            `json:"type"`      // "scale_up" or "scale_down"
	PodCount  int               `json:"pod_count"` // 대상 Pod 수
	Timestamp time.Time         `json:"timestamp"` // 이벤트 발생 시간
	Metadata  map[string]string `json:"metadata"`  // 추가 메타데이터
}

// NewClient는 새로운 Redis 클라이언트를 생성합니다.
//
// Parameters:
//   - addr: Redis 서버 주소 (예: "localhost:6379")
//   - password: Redis 패스워드 (빈 문자열이면 인증 없음)
//   - db: Redis 데이터베이스 번호 (일반적으로 0)
func NewClient(addr, password string, db int) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	return &Client{
		client: rdb,
	}
}

// Ping은 Redis 연결 상태를 확인합니다
func (c *Client) Ping(ctx context.Context) error {
	pong, err := c.client.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}
	log.Printf("✅ Redis connection successful: %s", pong)
	return nil
}

// ListenForScaleEvents는 Redis 스트림에서 스케일링 이벤트를 수신합니다.
//
// Consumer Group 패턴을 사용하여:
//   - 여러 Ottoscaler 인스턴스 간 이벤트 분산 처리
//   - 메시지 중복 처리 방지
//   - 자동 장애 복구 (다른 consumer가 처리)
//
// Returns:
//   - <-chan ScaleEvent: 이벤트를 수신할 채널
//   - error: 초기화 실패 시 에러
func (c *Client) ListenForScaleEvents(ctx context.Context, streamName, consumerGroup, consumer string) (<-chan ScaleEvent, error) {
	// Consumer Group 생성 (이미 존재하면 무시)
	err := c.client.XGroupCreateMkStream(ctx, streamName, consumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return nil, fmt.Errorf("failed to create consumer group '%s': %w", consumerGroup, err)
	}

	log.Printf("📡 Consumer Group '%s' ready for stream '%s'", consumerGroup, streamName)

	eventChan := make(chan ScaleEvent, EventChannelBuffer)

	// 백그라운드에서 이벤트 폴링 시작
	go c.pollEvents(ctx, streamName, consumerGroup, consumer, eventChan)

	return eventChan, nil
}

// pollEvents는 Redis 스트림을 지속적으로 폴링하여 이벤트를 수신합니다
func (c *Client) pollEvents(ctx context.Context, streamName, consumerGroup, consumer string, eventChan chan<- ScaleEvent) {
	defer close(eventChan)
	log.Printf("🔄 Started polling stream '%s' every %v", streamName, PollTimeout)

	for {
		select {
		case <-ctx.Done():
			log.Println("🛑 Event polling stopped")
			return
		default:
			if err := c.pollOnce(ctx, streamName, consumerGroup, consumer, eventChan); err != nil {
				log.Printf("⚠️ Polling error: %v, retrying in %v...", err, RetryDelay)

				select {
				case <-time.After(RetryDelay):
					// 재시도 대기
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

// pollOnce는 한 번의 폴링을 수행합니다
func (c *Client) pollOnce(ctx context.Context, streamName, consumerGroup, consumer string, eventChan chan<- ScaleEvent) error {
	streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    consumerGroup,
		Consumer: consumer,
		Streams:  []string{streamName, ">"},
		Count:    MaxMessagesPerPoll,
		Block:    PollTimeout,
	}).Result()

	if err != nil {
		if err == redis.Nil {
			// 새 메시지 없음 (정상)
			return nil
		}
		return fmt.Errorf("XReadGroup failed: %w", err)
	}

	// 수신된 메시지 처리
	for _, stream := range streams {
		for _, message := range stream.Messages {
			event, err := c.parseScaleEvent(message)
			if err != nil {
				log.Printf("❌ Failed to parse message %s: %v", message.ID, err)
				// 파싱 실패한 메시지도 ACK하여 재처리 방지
				c.client.XAck(ctx, streamName, consumerGroup, message.ID)
				continue
			}

			// 이벤트 전송
			select {
			case eventChan <- event:
				log.Printf("📨 Event forwarded: %s (ID: %s)", event.Type, event.EventID)
			case <-ctx.Done():
				return ctx.Err()
			}

			// 메시지 확인 (ACK)
			if err := c.client.XAck(ctx, streamName, consumerGroup, message.ID).Err(); err != nil {
				log.Printf("⚠️ Failed to ACK message %s: %v", message.ID, err)
			}
		}
	}

	return nil
}

// parseScaleEvent는 Redis 메시지를 ScaleEvent 구조체로 파싱합니다
func (c *Client) parseScaleEvent(message redis.XMessage) (ScaleEvent, error) {
	event := ScaleEvent{
		EventID:   message.ID,
		Metadata:  make(map[string]string),
		Timestamp: time.Now(), // 기본값, timestamp 필드가 있으면 덮어씀
	}

	// 필수 필드 검증용
	var hasType, hasPodCount bool

	// 메시지 필드 파싱
	for key, value := range message.Values {
		strValue, ok := value.(string)
		if !ok {
			continue
		}

		switch key {
		case "type":
			event.Type = strValue
			hasType = true
		case "pod_count":
			if count, err := strconv.Atoi(strValue); err == nil {
				event.PodCount = count
				hasPodCount = true
			} else {
				return event, fmt.Errorf("invalid pod_count value: %s", strValue)
			}
		case "timestamp":
			if ts, err := strconv.ParseInt(strValue, 10, 64); err == nil {
				event.Timestamp = time.Unix(ts, 0)
			}
		default:
			// 기타 필드는 메타데이터에 저장
			event.Metadata[key] = strValue
		}
	}

	// 필수 필드 검증
	if !hasType {
		return event, fmt.Errorf("missing required field: type")
	}
	if !hasPodCount {
		return event, fmt.Errorf("missing required field: pod_count")
	}

	return event, nil
}

// PublishScaleEvent는 스케일링 이벤트를 Redis 스트림에 발행합니다.
//
// 주로 테스트나 외부 시스템에서 스케일링을 트리거할 때 사용됩니다.
func (c *Client) PublishScaleEvent(ctx context.Context, streamName string, event ScaleEvent) error {
	values := map[string]interface{}{
		"type":      event.Type,
		"pod_count": fmt.Sprintf("%d", event.PodCount),
		"timestamp": event.Timestamp.Unix(),
	}

	// 메타데이터 추가
	for k, v := range event.Metadata {
		values[k] = v
	}

	result, err := c.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: values,
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	log.Printf("📤 Event published: %s (ID: %s)", event.Type, result)
	return nil
}

// Close는 Redis 클라이언트 연결을 종료합니다
func (c *Client) Close() error {
	if err := c.client.Close(); err != nil {
		return fmt.Errorf("failed to close Redis client: %w", err)
	}
	log.Println("🔌 Redis client connection closed")
	return nil
}
