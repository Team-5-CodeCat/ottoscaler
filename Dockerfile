# Ottoscaler - Multi-stage Docker Build
# 
# Stage 1: Build stage with full Go toolchain
# Stage 2: Runtime stage with minimal Alpine image

# ============================================================================
# Build Stage
# ============================================================================
FROM golang:1.24-alpine AS builder

# 메타데이터 라벨
LABEL stage=builder
LABEL description="Ottoscaler build environment"

# 작업 디렉토리 설정
WORKDIR /build

# 시스템 의존성 설치
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata

# Go 모듈 파일 복사 (캐싱 최적화)
COPY go.mod go.sum ./

# 의존성 다운로드 (레이어 캐싱)
RUN go mod download && go mod verify

# 소스 코드 복사
COPY . .

# 애플리케이션 빌드
# - CGO_ENABLED=0: 정적 바이너리 생성
# - GOOS=linux: Linux 타겟
# - -ldflags: 빌드 정보 및 최적화
RUN CGO_ENABLED=0 GOOS=linux go build \
    -a -installsuffix cgo \
    -ldflags='-w -s -extldflags "-static"' \
    -o ottoscaler \
    ./cmd/app

# ============================================================================
# Runtime Stage
# ============================================================================
FROM alpine:latest

# 메타데이터 라벨
LABEL maintainer="Team-5-CodeCat"
LABEL description="Ottoscaler - Kubernetes Native Auto-Scaler"
LABEL version="1.0.0"

# 런타임 의존성 설치
RUN apk --no-cache add \
    ca-certificates \
    tzdata

# 비root 사용자 생성
RUN addgroup -g 1001 -S ottoscaler && \
    adduser -u 1001 -S ottoscaler -G ottoscaler

# 작업 디렉토리 설정
WORKDIR /app

# 빌드된 바이너리 복사
COPY --from=builder /build/ottoscaler .

# 바이너리 실행 권한 설정
RUN chmod +x ottoscaler

# 사용자 변경
USER ottoscaler

# 헬스체크 포트 노출 (향후 사용)
EXPOSE 8080

# 헬스체크 설정 (향후 구현)
# HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
#     CMD ./ottoscaler --health-check

# 애플리케이션 실행
ENTRYPOINT ["./ottoscaler"]