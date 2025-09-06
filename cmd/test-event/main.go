// Package main provides a simple tool to send test events to Redis Streams
// for testing the Ottoscaler functionality.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/Team-5-CodeCat/ottoscaler/internal/redis"
)

func main() {
	log.Println("🧪 Redis Event Test Tool")

	// Check for ENV_FILE environment variable (required for multi-user setup)
	envFile := os.Getenv("ENV_FILE")
	if envFile == "" {
		log.Printf("❌ ENV_FILE environment variable is required")
		log.Printf("💡 Please specify your environment file:")
		log.Printf("   ENV_FILE=\".env.jinwoo.local\" go run ./cmd/test-event")
		log.Printf("   ENV_FILE=\".env.junyoung.local\" go run ./cmd/test-event")
		log.Printf("   또는: ENV_FILE=\".env.jinwoo.local\" make test-event")
		log.Printf("")
		log.Printf("🔧 Available developers:")
		log.Printf("   한진우 (jinwoo), 장준영 (junyoung), 고민지 (minji)")
		log.Printf("   이지윤 (jiyoon), 김보아 (boa), 유호준 (hojun)")
		log.Printf("")
		log.Printf("🚀 First time setup: make setup-user USER=한진우")
		os.Exit(1)
	}
	
	// Load the specified environment file
	if err := godotenv.Load(envFile); err != nil {
		log.Printf("❌ Failed to load environment file: %s", envFile)
		log.Printf("💡 Make sure the file exists or run: make setup-user USER=한진우")
		os.Exit(1)
	}
	
	log.Printf("📁 Loaded environment from: %s", envFile)

	// Configuration
	redisHost := getEnv("REDIS_HOST", "localhost")
	redisPort := getEnv("REDIS_PORT", "6379")
	redisPassword := getEnv("REDIS_PASSWORD", "")
	redisDB := getEnvInt("REDIS_DB", 0)
	redisStream := getEnv("REDIS_STREAM", "otto:scale:events")

	log.Printf("📡 Target: redis://%s:%s", redisHost, redisPort)
	log.Printf("📊 Stream: %s", redisStream)

	// Initialize Redis client
	redisAddr := fmt.Sprintf("%s:%s", redisHost, redisPort)
	redisClient := redis.NewClient(redisAddr, redisPassword, redisDB)

	ctx := context.Background()

	// Test connection
	if err := redisClient.Ping(ctx); err != nil {
		log.Fatalf("❌ Cannot connect to Redis: %v", err)
	}

	// Generate test event
	taskID := fmt.Sprintf("task-%d", time.Now().Unix())

	testEvent := redis.ScaleEvent{
		// --- 기본 스케일링 정보 ---
		Type:      "scale_up",
		Timestamp: time.Now(),

		// --- 스케일링 대상 및 수량 정보 ---
		TargetDeployment: "worker-deployment", // Example deployment name
		TargetReplicas:   5,                   // Example target replicas

		// --- CI/CD 작업 컨텍스트 정보 ---
		JobID:       fmt.Sprintf("job-%d", time.Now().Unix()),
		Repository:  "https://github.com/Team-5-CodeCat/ottoscaler.git",
		CommitSHA:   "a1b2c3d4e5f67890", // Example commit SHA
		TriggeredBy: "user:test-event-tool",

		// --- 운영 및 메타데이터 ---
		Reason: "Manual test event from test-tool",
		Metadata: map[string]string{
			"task_id":    taskID,
			"test_event": "true",
			"source":     "test-tool",
		},
	}

	log.Printf("📤 Sending test scale_up event...")
	log.Printf("   Task ID: %s", taskID)
	log.Printf("   Target Deployment: %s", testEvent.TargetDeployment)
	log.Printf("   Target Replicas: %d", testEvent.TargetReplicas)

	// Send event
	if err := redisClient.PublishScaleEvent(ctx, redisStream, testEvent); err != nil {
		log.Fatalf("❌ Failed to send event: %v", err)
	}

	log.Println("✅ Event sent successfully!")
	log.Println("🎯 Monitor your Ottoscaler logs to see event processing.")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}