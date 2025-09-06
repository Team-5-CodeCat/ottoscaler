# Changelog

## [2.0.0] - 2025-01-06

### 🚀 Major Architecture Change
- **BREAKING**: Migrated from Redis Streams to gRPC architecture
- Direct communication between otto-handler and ottoscaler
- Removed all Redis dependencies

### ✨ New Features
- **Pipeline Execution Support**
  - DAG-based dependency resolution
  - Parallel stage execution
  - Real-time progress streaming via gRPC
  - Stage retry policies
  - Created `internal/pipeline/executor.go`

- **Test Tools**
  - `cmd/test-pipeline/main.go`: Pipeline execution testing
  - Support for simple, full, and parallel pipeline types
  - Real-time progress visualization

### 🔄 Changes
- **Korean Localization**
  - All log messages converted to Korean
  - Added developer-friendly emojis
  - Better local developer experience

- **Code Modernization**
  - Updated deprecated gRPC patterns:
    - `grpc.WithInsecure()` → `grpc.WithTransportCredentials(insecure.NewCredentials())`
    - `grpc.DialContext()` → `grpc.NewClient()`
    - Removed `grpc.WithBlock()` (not supported in NewClient)

### 🗑️ Removed
- `internal/redis/client.go` - Redis client implementation
- `cmd/test-event/main.go` - Redis event testing tool
- `configs/README.md` - Outdated configuration docs
- `scripts/test-redis-event.sh` - Redis testing script
- All Redis-related configurations from Makefile

### 📚 Documentation
- Created comprehensive `docs/PROJECT_ANALYSIS.md`
- Updated README.md with current architecture
- Added CHANGELOG.md (this file)
- Updated CLAUDE.md with current guidelines

### 🐛 Bug Fixes
- Fixed ENV_FILE requirement in Kubernetes environment
- Fixed unused variable warnings
- Fixed gRPC connection handling

### 🏗️ Infrastructure
- Simplified deployment without Redis dependency
- Improved Kind cluster integration
- Enhanced multi-developer environment support

### 📦 Dependencies
- Updated to latest gRPC and protobuf versions
- Cleaned up unused dependencies

## [1.0.0] - 2024-12-15

### Initial Release
- Redis Streams based event processing
- Basic worker pod management
- Multi-developer environment support
- gRPC service skeleton