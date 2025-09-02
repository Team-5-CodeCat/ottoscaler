.PHONY: help dev-build dev-start dev-shell dev-stop dev-clean redis-cli test-event proto test fmt lint build deploy logs clean

# 변수 정의
DEV_CONTAINER_NAME := ottoscaler-dev
REDIS_CONTAINER_NAME := ottoscaler-redis
DEV_IMAGE_NAME := ottoscaler-dev
PROD_IMAGE_NAME := ottoscaler
VERSION := latest

# 색상 정의
GREEN := \033[0;32m
YELLOW := \033[1;33m
BLUE := \033[0;34m
RED := \033[0;31m
NC := \033[0m # No Color

# 기본 타겟
help:
	@echo "$(GREEN)🚀 Ottoscaler - 크로스 플랫폼 개발 환경$(NC)"
	@echo ""
	@echo "$(BLUE)📦 개발 환경:$(NC)"
	@echo "  dev-build   - 개발 환경 이미지 빌드"
	@echo "  dev-start   - 개발 환경 시작 (Redis + Dev Container)"
	@echo "  dev-shell   - 개발 컨테이너에 접속"
	@echo "  dev-stop    - 개발 환경 중지"
	@echo "  dev-clean   - 개발 환경 완전 정리"
	@echo ""
	@echo "$(BLUE)🔧 개발 도구:$(NC)"
	@echo "  proto       - Protocol Buffer 코드 생성"
	@echo "  test        - 테스트 실행"
	@echo "  fmt         - 코드 포맷팅"
	@echo "  lint        - 코드 린트"
	@echo ""
	@echo "$(BLUE)🎯 테스트 & 디버깅:$(NC)"
	@echo "  test-event  - Redis에 테스트 이벤트 전송"
	@echo "  redis-cli   - Redis CLI 접속"
	@echo ""
	@echo "$(BLUE)🏭 프로덕션:$(NC)"
	@echo "  build       - 프로덕션 이미지 빌드"
	@echo "  deploy      - Kubernetes 배포"
	@echo "  logs        - 배포된 Pod 로그 조회"
	@echo ""
	@echo "$(BLUE)🧹 정리:$(NC)"
	@echo "  clean       - 모든 리소스 정리"

# 개발 환경 관리
dev-build:
	@echo "$(YELLOW)🏗️ 개발 환경 이미지 빌드 중...$(NC)"
	@docker build -f dev.Dockerfile -t $(DEV_IMAGE_NAME):$(VERSION) .
	@echo "$(GREEN)✅ 개발 환경 이미지 빌드 완료!$(NC)"

dev-start: dev-build redis-start
	@echo "$(YELLOW)🚀 개발 환경 시작 중...$(NC)"
	@if docker ps -a --format '{{.Names}}' | grep -q "^$(DEV_CONTAINER_NAME)$$"; then \
		if ! docker ps --format '{{.Names}}' | grep -q "^$(DEV_CONTAINER_NAME)$$"; then \
			echo "$(BLUE)🔄 기존 개발 컨테이너 시작...$(NC)"; \
			docker start $(DEV_CONTAINER_NAME); \
		else \
			echo "$(BLUE)✅ 개발 컨테이너가 이미 실행 중$(NC)"; \
		fi \
	else \
		echo "$(YELLOW)🆕 새 개발 컨테이너 생성...$(NC)"; \
		docker run -d \
			--name $(DEV_CONTAINER_NAME) \
			--network host \
			-v $(PWD):/workspace \
			-v ottoscaler-go-cache:/go/pkg/mod \
			-v $(HOME)/.kube:/root/.kube:ro \
			-v /var/run/docker.sock:/var/run/docker.sock \
			-w /workspace \
			-e REDIS_HOST=localhost \
			-e REDIS_PORT=6379 \
			-e KUBECONFIG=/root/.kube/config \
			$(DEV_IMAGE_NAME):$(VERSION) \
			tail -f /dev/null; \
		echo "$(GREEN)✅ 개발 컨테이너 생성 완료$(NC)"; \
	fi
	@echo ""
	@echo "$(GREEN)🎉 개발 환경 준비 완료!$(NC)"
	@echo ""
	@echo "$(BLUE)다음 단계:$(NC)"
	@echo "  make dev-shell  # 개발 컨테이너 접속"
	@echo "  # (컨테이너 내부에서)"
	@echo "  go run ./cmd/app  # 애플리케이션 실행"

dev-shell:
	@echo "$(BLUE)🐚 개발 컨테이너에 접속합니다...$(NC)"
	@if ! docker ps --format '{{.Names}}' | grep -q "^$(DEV_CONTAINER_NAME)$$"; then \
		echo "$(RED)❌ 개발 컨테이너가 실행되지 않았습니다.$(NC)"; \
		echo "$(YELLOW)먼저 'make dev-start'를 실행하세요.$(NC)"; \
		exit 1; \
	fi
	@docker exec -it $(DEV_CONTAINER_NAME) /bin/bash

dev-stop:
	@echo "$(YELLOW)⏹️ 개발 환경 중지 중...$(NC)"
	@docker stop $(DEV_CONTAINER_NAME) $(REDIS_CONTAINER_NAME) 2>/dev/null || echo "$(BLUE)개발 컨테이너 중지$(NC)"
	@echo "$(GREEN)✅ 개발 환경 중지 완료$(NC)"

dev-clean:
	@echo "$(RED)🧹 개발 환경 완전 정리 중...$(NC)"
	@echo "$(YELLOW)⚠️ 이 작업은 개발 컨테이너와 캐시를 모두 삭제합니다!$(NC)"
	@read -p "계속하시겠습니까? (y/N): " confirm && [ "$$confirm" = "y" ]
	@docker stop $(DEV_CONTAINER_NAME) $(REDIS_CONTAINER_NAME) 2>/dev/null || true
	@docker rm $(DEV_CONTAINER_NAME) $(REDIS_CONTAINER_NAME) 2>/dev/null || true
	@docker volume rm ottoscaler-go-cache 2>/dev/null || true
	@docker rmi $(DEV_IMAGE_NAME):$(VERSION) 2>/dev/null || true
	@echo "$(GREEN)✅ 개발 환경 완전 정리 완료$(NC)"

# Redis 관리 (dev-start에 통합됨)
redis-start:
	@echo "$(YELLOW)🗄️ Redis 시작 중...$(NC)"
	@if docker ps -a --format '{{.Names}}' | grep -q "^$(REDIS_CONTAINER_NAME)$$"; then \
		if ! docker ps --format '{{.Names}}' | grep -q "^$(REDIS_CONTAINER_NAME)$$"; then \
			echo "$(BLUE)🔄 기존 Redis 컨테이너 시작...$(NC)"; \
			docker start $(REDIS_CONTAINER_NAME); \
		else \
			echo "$(BLUE)✅ Redis가 이미 실행 중$(NC)"; \
		fi \
	else \
		echo "$(YELLOW)🆕 새 Redis 컨테이너 생성...$(NC)"; \
		docker run -d --name $(REDIS_CONTAINER_NAME) \
			-p 6379:6379 \
			redis:7-alpine redis-server --appendonly yes; \
		echo "$(GREEN)✅ Redis 컨테이너 생성 완료$(NC)"; \
	fi

redis-cli:
	@echo "$(BLUE)💻 Redis CLI 접속...$(NC)"
	@if [ -f /.dockerenv ]; then \
		redis-cli -h localhost; \
	elif docker ps --format '{{.Names}}' | grep -q "^$(REDIS_CONTAINER_NAME)$$"; then \
		docker exec -it $(REDIS_CONTAINER_NAME) redis-cli; \
	else \
		echo "$(RED)❌ Redis가 실행되지 않았습니다.$(NC)"; \
		echo "$(YELLOW)먼저 'make dev-start'를 실행하세요.$(NC)"; \
		exit 1; \
	fi

# 개발 도구 (호스트/컨테이너 양쪽에서 사용 가능)
proto:
	@echo "$(YELLOW)🔧 Protocol Buffer 코드 생성 중...$(NC)"
	@if [ -f /.dockerenv ]; then \
		./scripts/generate-proto.sh; \
	elif docker ps --format '{{.Names}}' | grep -q "^$(DEV_CONTAINER_NAME)$$"; then \
		docker exec $(DEV_CONTAINER_NAME) /bin/bash -c "cd /workspace && ./scripts/generate-proto.sh"; \
	else \
		echo "$(RED)❌ 개발 컨테이너가 실행되지 않았습니다.$(NC)"; \
		echo "$(YELLOW)먼저 'make dev-start'를 실행하세요.$(NC)"; \
		exit 1; \
	fi

test:
	@echo "$(YELLOW)🧪 테스트 실행 중...$(NC)"
	@if [ -f /.dockerenv ]; then \
		go test ./...; \
	elif docker ps --format '{{.Names}}' | grep -q "^$(DEV_CONTAINER_NAME)$$"; then \
		docker exec $(DEV_CONTAINER_NAME) /bin/bash -c "cd /workspace && go test ./..."; \
	else \
		echo "$(RED)❌ 개발 컨테이너가 실행되지 않았습니다.$(NC)"; \
		echo "$(YELLOW)먼저 'make dev-start'를 실행하세요.$(NC)"; \
		exit 1; \
	fi

fmt:
	@echo "$(YELLOW)🎨 코드 포맷팅 중...$(NC)"
	@if [ -f /.dockerenv ]; then \
		go fmt ./...; \
	elif docker ps --format '{{.Names}}' | grep -q "^$(DEV_CONTAINER_NAME)$$"; then \
		docker exec $(DEV_CONTAINER_NAME) /bin/bash -c "cd /workspace && go fmt ./..."; \
	else \
		echo "$(RED)❌ 개발 컨테이너가 실행되지 않았습니다.$(NC)"; \
		echo "$(YELLOW)먼저 'make dev-start'를 실행하세요.$(NC)"; \
		exit 1; \
	fi

lint:
	@echo "$(YELLOW)🔍 코드 린트 중...$(NC)"
	@if [ -f /.dockerenv ]; then \
		golangci-lint run; \
	elif docker ps --format '{{.Names}}' | grep -q "^$(DEV_CONTAINER_NAME)$$"; then \
		docker exec $(DEV_CONTAINER_NAME) /bin/bash -c "cd /workspace && golangci-lint run"; \
	else \
		echo "$(RED)❌ 개발 컨테이너가 실행되지 않았습니다.$(NC)"; \
		echo "$(YELLOW)먼저 'make dev-start'를 실행하세요.$(NC)"; \
		exit 1; \
	fi

# 테스트 & 디버깅
test-event:
	@echo "$(YELLOW)📤 테스트 이벤트 전송 중...$(NC)"
	@TIMESTAMP=$$(date +%s); \
	if [ -f /.dockerenv ]; then \
		redis-cli -h localhost XADD otto:scale:events '*' \
			type scale_up pod_count 3 task_id test-$$TIMESTAMP timestamp $$TIMESTAMP; \
	elif docker ps --format '{{.Names}}' | grep -q "^$(REDIS_CONTAINER_NAME)$$"; then \
		docker exec $(REDIS_CONTAINER_NAME) redis-cli XADD otto:scale:events '*' \
			type scale_up pod_count 3 task_id test-$$TIMESTAMP timestamp $$TIMESTAMP; \
	else \
		echo "$(RED)❌ Redis가 실행되지 않았습니다.$(NC)"; \
		echo "$(YELLOW)먼저 'make dev-start'를 실행하세요.$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)✅ 테스트 이벤트 전송 완료!$(NC)"

# 프로덕션
build:
	@echo "$(YELLOW)🏗️ 프로덕션 이미지 빌드 중...$(NC)"
	@docker build -t $(PROD_IMAGE_NAME):$(VERSION) .
	@echo "$(GREEN)✅ 프로덕션 이미지 빌드 완료!$(NC)"

deploy: build
	@echo "$(YELLOW)🚀 Kubernetes 배포 중...$(NC)"
	@kubectl apply -f k8s/rbac.yaml
	@kubectl apply -f k8s/deployment.yaml
	@echo "$(BLUE)⏳ 배포 완료 대기 중...$(NC)"
	@kubectl wait --for=condition=available --timeout=120s deployment/ottoscaler
	@echo "$(GREEN)✅ 배포 완료!$(NC)"

logs:
	@echo "$(BLUE)📋 Pod 로그 조회 (Ctrl+C로 종료):$(NC)"
	@kubectl logs -l app=ottoscaler -f --tail=50

# 정리
clean:
	@echo "$(RED)🧹 모든 리소스 정리 중...$(NC)"
	@echo "$(YELLOW)⚠️ 이 작업은 모든 컨테이너, 이미지, 볼륨을 삭제합니다!$(NC)"
	@read -p "계속하시겠습니까? (y/N): " confirm && [ "$$confirm" = "y" ]
	@echo "개발 환경 정리..."
	@docker stop $(DEV_CONTAINER_NAME) $(REDIS_CONTAINER_NAME) 2>/dev/null || true
	@docker rm $(DEV_CONTAINER_NAME) $(REDIS_CONTAINER_NAME) 2>/dev/null || true
	@echo "Kubernetes 리소스 정리..."
	@kubectl delete -f k8s/deployment.yaml --ignore-not-found 2>/dev/null || true
	@kubectl delete -f k8s/rbac.yaml --ignore-not-found 2>/dev/null || true
	@kubectl delete pods -l managed-by=ottoscaler --ignore-not-found 2>/dev/null || true
	@echo "Docker 리소스 정리..."
	@docker rmi $(DEV_IMAGE_NAME):$(VERSION) $(PROD_IMAGE_NAME):$(VERSION) 2>/dev/null || true
	@docker volume rm ottoscaler-go-cache 2>/dev/null || true
	@docker system prune -f >/dev/null 2>&1 || true
	@echo "$(GREEN)✅ 전체 정리 완료!$(NC)"