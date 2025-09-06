.PHONY: help setup-user test fmt lint build deploy logs clean proto install-deps dev-start dev-stop k8s-status run-app

# 변수 정의
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
	@echo "$(GREEN)🚀 Ottoscaler - 멀티 유저 개발 환경$(NC)"
	@echo ""
	@echo "$(BLUE)👥 멀티 유저 환경:$(NC)"
	@echo "  setup-user USER=한진우  - 개발자별 환경 자동 구성"
	@echo ""
	@echo "$(BLUE)🔧 개발 도구:$(NC)"
	@echo "  test-scaling - gRPC 테스트 클라이언트 빌드 및 실행"
	@echo "  test        - 테스트 실행"
	@echo "  fmt         - 코드 포맷팅"
	@echo "  lint        - 코드 린트"
	@echo "  proto       - Protocol Buffer 코드 생성 (TODO: gRPC 구현 시)"
	@echo "  install-deps - Go 의존성 설치 및 정리"
	@echo ""
	@echo "$(BLUE)🏭 배포:$(NC)"
	@echo "  build       - 이미지 빌드"
	@echo "  deploy      - Kind 클러스터에 Main Pod로 배포"
	@echo "  logs        - Main Pod 로그 조회"
	@echo ""
	@echo "$(BLUE)🛠️ 유틸리티:$(NC)"
	@echo "  port-forward - gRPC 서버 포트 포워딩 (9090)"
	@echo "  k8s-status  - Kubernetes 클러스터 상태 확인"
	@echo "  status      - 전체 환경 상태 확인"
	@echo "  list-envs   - 사용 가능한 환경 파일 목록"
	@echo ""
	@echo "$(BLUE)🧹 정리:$(NC)"
	@echo "  clean       - 모든 리소스 정리 (Redis 컨테이너, Kind 클러스터)"
	@echo "  dev-stop    - 개발 환경 중지 (clean과 동일)"
	@echo ""
	@echo "$(YELLOW)💡 사용법:$(NC)"
	@echo "  1. make setup-user USER=한진우                      # 환경 설정 (최초 1회)"
	@echo "  2. make build && make deploy                        # Main Pod 배포"
	@echo "  3. make port-forward                                # 포트 포워딩"
	@echo "  4. ./test-scaling -action scale-up -workers 3       # 테스트"
	@echo ""
	@echo "$(GREEN)🎯 개발자별 환경:$(NC)"
	@echo "  한진우: ENV_FILE='.env.hanjinwoo.local'"
	@echo "  장준영: ENV_FILE='.env.jangjunyoung.local'"
	@echo "  고민지: ENV_FILE='.env.gominji.local'"
	@echo "  이지윤: ENV_FILE='.env.leejiyun.local'"
	@echo "  김보아: ENV_FILE='.env.kimboa.local'"
	@echo "  유호준: ENV_FILE='.env.yoohojun.local'"

# 다중 사용자 환경 설정
setup-user:
	@if [ -z "$(USER)" ]; then \
		echo "$(RED)❌ Error: USER parameter is required$(NC)"; \
		echo "$(YELLOW)Usage: make setup-user USER=한진우$(NC)"; \
		echo "$(BLUE)Available users: 한진우, 장준영, 고민지, 이지윤, 김보아, 유호준$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)🚀 Setting up environment for: $(USER)$(NC)"
	@./scripts/setup-user-env.sh "$(USER)"

# 로컬 개발용 (참고용 - 실제로는 Main Pod로 배포하여 개발)
run-app:
	@echo "$(YELLOW)⚠️ 참고: 로컬 실행은 개발 편의용입니다.$(NC)"
	@echo "$(BLUE)실제 개발은 'make build && make deploy'로 Main Pod를 배포하여 진행하세요.$(NC)"
	@if [ -n "$(ENV_FILE)" ]; then \
		echo "$(BLUE)📁 Using environment file: $(ENV_FILE)$(NC)"; \
		ENV_FILE=$(ENV_FILE) go run ./cmd/app; \
	else \
		echo "$(RED)❌ ENV_FILE environment variable is required$(NC)"; \
		echo "$(YELLOW)Usage: ENV_FILE='.env.hanjinwoo.local' make run-app$(NC)"; \
		echo "$(BLUE)Available environments:$(NC)"; \
		echo "  ENV_FILE='.env.hanjinwoo.local' make run-app"; \
		echo "  ENV_FILE='.env.jangjunyoung.local' make run-app"; \
		echo "  ENV_FILE='.env.gominji.local' make run-app"; \
		echo "$(YELLOW)First time setup: make setup-user USER=한진우$(NC)"; \
		exit 1; \
	fi

# 테스트 & 디버깅
test-scaling:
	@echo "$(YELLOW)🔨 테스트 클라이언트 빌드 중...$(NC)"
	@go build -o test-scaling ./cmd/test-scaling
	@echo "$(GREEN)✅ 테스트 클라이언트 빌드 완료!$(NC)"
	@echo "$(BLUE)사용법:$(NC)"
	@echo "  ./test-scaling -action scale-up -workers 3"
	@echo "  ./test-scaling -action status"
	@echo "  ./test-scaling -h  # 도움말"

port-forward:
	@echo "$(YELLOW)🔌 gRPC 서버 포트 포워딩 (9090)...$(NC)"
	@kubectl port-forward deployment/ottoscaler 9090:9090

# 개발 도구
test:
	@echo "$(YELLOW)🧪 테스트 실행 중...$(NC)"
	@go test -v -race ./...
	@echo "$(GREEN)✅ 테스트 완료!$(NC)"

fmt:
	@echo "$(YELLOW)🎨 코드 포맷팅 중...$(NC)"
	@go fmt ./...
	@echo "$(GREEN)✅ 코드 포맷팅 완료!$(NC)"

lint:
	@echo "$(YELLOW)🔍 코드 린트 중...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "$(YELLOW)⚠️ golangci-lint not found, using go vet instead$(NC)"; \
		go vet ./...; \
	fi
	@echo "$(GREEN)✅ 코드 린트 완료!$(NC)"

# Protocol Buffer 코드 생성
proto:
	@echo "$(YELLOW)🔨 Protocol Buffer 코드 생성 중...$(NC)"
	@if [ -f scripts/generate-proto.sh ]; then \
		./scripts/generate-proto.sh; \
	else \
		echo "$(RED)❌ scripts/generate-proto.sh not found$(NC)"; \
		echo "$(YELLOW) Protocol Buffer 스크립트를 먼저 생성하세요$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)✅ Protocol Buffer 코드 생성 완료!$(NC)"

# 의존성 관리
install-deps:
	@echo "$(YELLOW)🔧 Go 의존성 설치 중...$(NC)"
	@go mod download
	@go mod tidy
	@echo "$(GREEN)✅ 의존성 설치 완료!$(NC)"

# 개발 환경 관리 (참고용)
dev-start:
	@echo "$(YELLOW)🚀 개발 환경 시작 중...$(NC)"
	@echo "$(BLUE) 각 개발자는 'make setup-user USER=한진우'로 환경을 설정하세요$(NC)"
	@echo "$(BLUE)📋 Quick Start:$(NC)"
	@echo "  1. make setup-user USER=한진우"
	@echo "  2. make build && make deploy"
	@echo "  3. make port-forward  # 별도 터미널에서"
	@echo "  4. ./test-scaling -action scale-up -workers 3"

dev-stop:
	@echo "$(YELLOW)🛑 개발 환경 중지 중...$(NC)"
	@make clean
	@echo "$(GREEN)✅ 개발 환경 중지 완료!$(NC)"

# Worker Pod 확인
worker-status:
	@echo "$(YELLOW)📊 Worker Pod 상태 확인...$(NC)"
	@kubectl get pods -l managed-by=ottoscaler --all-namespaces
	@echo ""
	@echo "$(BLUE)💡 실시간 모니터링: kubectl get pods -w$(NC)"

# Kubernetes 상태 확인
k8s-status:
	@echo "$(GREEN)☸️ Kubernetes 클러스터 상태$(NC)"
	@echo ""
	@echo "$(BLUE) 현재 컨텍스트:$(NC)"
	@kubectl config current-context 2>/dev/null | sed 's/^/  /' || echo "  컨텍스트가 설정되지 않음"
	@echo ""
	@echo "$(BLUE)🏷️ 사용 가능한 컨텍스트:$(NC)"
	@kubectl config get-contexts --output=name 2>/dev/null | grep "ottoscaler-" | sed 's/^/  /' || echo "  Ottoscaler 컨텍스트 없음"
	@echo ""
	@echo "$(BLUE)📦 Pod 상태:$(NC)"
	@kubectl get pods --all-namespaces | grep ottoscaler || echo "  Ottoscaler Pod 없음"
	@echo ""
	@echo "$(BLUE)️ 네임스페이스:$(NC)"
	@kubectl get namespaces | grep -E "(jinwoo|junyoung|minji|jiyoon|boa|hojun)" | sed 's/^/  /' || echo "  개발자 네임스페이스 없음"

# 배포
build:
	@echo "$(YELLOW)🏗️ 이미지 빌드 중...$(NC)"
	@docker build -t $(PROD_IMAGE_NAME):$(VERSION) .
	@echo "$(GREEN)✅ 이미지 빌드 완료: $(PROD_IMAGE_NAME):$(VERSION)$(NC)"

deploy:
	@echo "$(YELLOW)🚀 Kind 클러스터에 Main Pod 배포 중...$(NC)"
	@if [ ! -f k8s/deployment.yaml ]; then \
		echo "$(RED)❌ k8s/deployment.yaml not found$(NC)"; \
		exit 1; \
	fi
	@kubectl apply -f k8s/
	@echo "$(GREEN)✅ Main Pod 배포 완료!$(NC)"
	@echo "$(BLUE)📊 배포 상태 확인:$(NC)"
	@kubectl get pods -l app=ottoscaler

logs:
	@echo "$(YELLOW)📄 Main Pod 로그 조회...$(NC)"
	@kubectl logs -l app=ottoscaler -f --tail=100

# 정리
clean:
	@echo "$(YELLOW)🧹 리소스 정리 중...$(NC)"
	@echo "$(BLUE)☸️ Kind 클러스터 정리...$(NC)"
	@kind get clusters | grep "ottoscaler-" | xargs -I {} kind delete cluster --name {} 2>/dev/null || true
	@echo "$(BLUE)📁 환경 파일 정리...$(NC)"
	@rm -f .env.*.local
	@echo "$(BLUE)🐳 Docker 이미지 정리...$(NC)"
	@docker images | grep -E "(ottoscaler|<none>)" | awk '{print $$3}' | xargs -r docker rmi -f 2>/dev/null || true
	@echo "$(GREEN)✅ 정리 완료!$(NC)"

# 유틸리티 함수들
.PHONY: check-env-file
check-env-file:
	@if [ -z "$(ENV_FILE)" ]; then \
		echo "$(RED)❌ ENV_FILE environment variable is required$(NC)"; \
		echo "$(YELLOW)Usage: ENV_FILE='.env.jinwoo.local' make <command>$(NC)"; \
		exit 1; \
	fi

.PHONY: list-envs
list-envs:
	@echo "$(BLUE)📁 Available environment files:$(NC)"
	@ls -1 .env.*.local 2>/dev/null || echo "$(YELLOW)⚠️ No environment files found. Run 'make setup-user USER=한진우' first.$(NC)"

.PHONY: status
status:
	@echo "$(GREEN)🔍 Multi-User Environment Status$(NC)"
	@echo ""
	@echo "$(BLUE)📁 Environment Files:$(NC)"
	@ls -1 .env.*.local 2>/dev/null || echo "  No environment files found"
	@echo ""
	@echo "$(BLUE)🗄️ Redis Containers:$(NC)"
	@docker ps --filter "name=ottoscaler-redis-" --format "  {{.Names}} ({{.Status}}) - Port: {{.Ports}}" 2>/dev/null || echo "  No Redis containers running"
	@echo ""
	@echo "$(BLUE)☸️ Kind Clusters:$(NC)"
	@kind get clusters 2>/dev/null | grep "ottoscaler-" | sed 's/^/  /' || echo "  No Kind clusters found"
	@echo ""
	@echo "$(BLUE)📊 Current kubectl context:$(NC)"
	@kubectl config current-context 2>/dev/null | sed 's/^/  /' || echo "  No kubectl context set"
	@echo ""
	@echo "$(BLUE)🏗️ 개발자 네임스페이스:$(NC)"
	@kubectl get namespaces 2>/dev/null | grep -E "(hanjinwoo|jangjunyoung|gominji|leejiyun|kimboa|yoohojun)" | sed 's/^/  /' || echo "  개발자 네임스페이스 없음"