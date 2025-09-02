#!/bin/bash

# Protocol Buffer code generation script for Ottoscaler

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Generating Protocol Buffer code for Ottoscaler...${NC}"

# Check if protoc is installed
if ! command -v protoc &> /dev/null; then
    echo -e "${RED}Error: protoc is not installed${NC}"
    echo "Please install Protocol Buffers compiler:"
    echo "  macOS: brew install protobuf"
    echo "  Linux: apt-get install -y protobuf-compiler"
    exit 1
fi

# Check if protoc-gen-go is installed
if ! command -v protoc-gen-go &> /dev/null; then
    echo -e "${YELLOW}Installing protoc-gen-go...${NC}"
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

# Check if protoc-gen-go-grpc is installed
if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo -e "${YELLOW}Installing protoc-gen-go-grpc...${NC}"
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

# Create output directories
mkdir -p pkg/proto/v1

# Generate Go code from proto files
echo -e "${YELLOW}Generating Go code...${NC}"
protoc \
    --proto_path=proto \
    --go_out=pkg/proto/v1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=pkg/proto/v1 \
    --go-grpc_opt=paths=source_relative \
    proto/*.proto

echo -e "${GREEN}✅ Protocol Buffer code generation completed!${NC}"
echo -e "Generated files:"
find pkg/proto/v1 -name "*.go" | sed 's/^/  /'

# Optional: Generate TypeScript/JavaScript for NestJS (if needed)
if command -v protoc-gen-ts &> /dev/null; then
    echo -e "${YELLOW}Generating TypeScript code...${NC}"
    mkdir -p generated/typescript
    protoc \
        --proto_path=proto \
        --ts_out=generated/typescript \
        proto/*.proto
    echo -e "${GREEN}✅ TypeScript code generated!${NC}"
else
    echo -e "${YELLOW}Note: protoc-gen-ts not found, skipping TypeScript generation${NC}"
    echo "For NestJS integration, consider installing: npm install -g protoc-gen-ts"
fi

echo -e "${GREEN}🎉 All done!${NC}"