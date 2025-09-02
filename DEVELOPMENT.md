# Development Environment

이 문서는 모든 IDE와 운영체제에서 동일한 Docker 환경으로 Ottoscaler를 개발할 수 있도록 안내합니다.

## Prerequisites

- Docker
- Docker Compose
- Make (선택사항, 명령어 단순화용)

## 🚀 Quick Start

### 방법 1: 스크립트 사용 (가장 간단)
```bash
# 개발 환경 시작
./scripts/dev.sh

# 개발 컨테이너 진입
./scripts/shell.sh

# 컨테이너 내부에서 앱 실행
go run ./cmd/app
```

### 방법 2: Make 사용
```bash
# 개발 환경 시작
make dev

# 개발 컨테이너 진입
make shell

# 컨테이너 내부에서 앱 실행
go run ./cmd/app
```

### 방법 3: Docker Compose 직접 사용
```bash
# 개발 환경 시작
docker-compose --profile dev up -d

# 컨테이너 진입
docker-compose --profile dev exec ottoscaler-dev sh

# 앱 실행
go run ./cmd/app
```

## 📦 포함된 도구들

개발 컨테이너에는 다음이 설치되어 있습니다:
- **Go 1.24** + 모든 표준 도구
- **golangci-lint** - 코드 품질 검사
- **Protocol Buffers** 컴파일러 + Go 플러그인
- **Docker CLI & Docker Compose**
- **kubectl & helm** - Kubernetes 도구

## 🛠️ 개발 워크플로우

1. **환경 시작**: `./scripts/dev.sh` 또는 `make dev`
2. **컨테이너 진입**: `./scripts/shell.sh` 또는 `make shell`
3. **코드 작성**: 모든 IDE에서 로컬 파일 편집
4. **앱 실행**: 컨테이너 내부에서 `go run ./cmd/app`
5. **테스트**: `go test ./...`
6. **포매팅**: `go fmt ./...`

## 🔧 유용한 명령어

| 작업 | Make | 스크립트 | Docker Compose |
|------|------|----------|----------------|
| 개발환경 시작 | `make dev` | `./scripts/dev.sh` | `docker-compose --profile dev up -d` |
| 컨테이너 진입 | `make shell` | `./scripts/shell.sh` | `docker-compose exec ottoscaler-dev sh` |
| Redis CLI | `make redis-cli` | - | `docker-compose --profile tools run redis-cli` |
| 환경 정리 | `make clean` | `./scripts/clean.sh` | `docker-compose down -v` |

## IDE별 추가 설정

### VS Code 사용자
- `.vscode/extensions.json`에서 권장 확장 프로그램 자동 설치 제안
- `.vscode/settings.json`에서 Go 개발에 최적화된 설정 적용
- 저장 시 자동 포맷팅 (`gofmt`) 활성화

### 기타 IDE 사용자
모든 개발은 Docker 컨테이너 내부에서 이루어지므로 IDE에 관계없이 동일한 환경에서 작업 가능

### 1. 기본 개발 환경 실행

```bash
# Redis와 함께 개발 환경 시작
docker-compose --profile dev up ottoscaler-dev redis

# 백그라운드 실행
docker-compose --profile dev up -d ottoscaler-dev redis
```

### 2. Production 빌드 테스트

```bash
# 앱 빌드 및 실행
docker-compose up --build ottoscaler redis
```

### 3. Redis 디버깅

```bash
# Redis CLI 접속
docker-compose --profile tools run redis-cli

# Redis 컨테이너에 직접 접속
docker exec -it ottoscaler-redis redis-cli
```

## 개발 워크플로우

### 코드 변경 시

개발 모드(`ottoscaler-dev`)에서는 소스 코드가 마운트되어 있어 파일 변경 시 컨테이너를 다시 시작하면 변경사항이 반영됩니다:

```bash
# 컨테이너 재시작
docker-compose restart ottoscaler-dev
```

### Redis Streams 테스트

Redis CLI를 통해 메시지 큐 테스트:

```bash
# Redis CLI 접속
docker-compose --profile tools run redis-cli

# 스트림에 메시지 추가 (예시)
XADD events * action scale pod otto-agent replicas 3

# 스트림 확인
XLEN events
XRANGE events - +
```

## 서비스 설명

- **redis**: Redis 서버 (포트 6379)
- **ottoscaler**: 프로덕션 빌드된 애플리케이션
- **ottoscaler-dev**: 개발용 Go 환경 (소스 코드 마운트)
- **redis-cli**: Redis 디버깅용 CLI 도구

## 환경 변수

- `REDIS_HOST`: Redis 호스트 (기본값: redis)
- `REDIS_PORT`: Redis 포트 (기본값: 6379)

## 데이터 지속성

Redis 데이터는 `redis_data` Docker volume에 저장되어 컨테이너 재시작 시에도 유지됩니다.

## 정리

```bash
# 모든 컨테이너 및 네트워크 정리
docker-compose down

# 볼륨까지 완전 정리
docker-compose down -v
```