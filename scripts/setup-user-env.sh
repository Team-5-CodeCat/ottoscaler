#!/bin/bash

# Multi-User Development Environment Setup Script
# 여러 개발자를 위한 개발 환경 설정 스크립트

set -e

# 색상 정의
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 개발자 목록 (포트 자동 배정을 위한 순서)
declare -A DEVELOPERS=(
    ["한진우"]=1
    ["장준영"]=2
    ["고민지"]=3
    ["이지윤"]=4
    ["김보아"]=5
    ["유호준"]=6
)

# 사용법 출력
usage() {
    echo -e "${BLUE}🚀 Ottoscaler Multi-User Environment Setup${NC}"
    echo ""
    echo "Usage: $0 <developer_name>"
    echo ""
    echo -e "${YELLOW}Available developers:${NC}"
    for name in "${!DEVELOPERS[@]}"; do
        echo "  - $name"
    done
    echo ""
    echo -e "${BLUE}Examples:${NC}"
    echo "  $0 한진우"
    echo "  $0 장준영"
    exit 1
}

# 인자 검증
if [ $# -ne 1 ]; then
    echo -e "${RED}❌ Error: Developer name is required${NC}"
    usage
fi

DEVELOPER_NAME="$1"

# 개발자 이름 검증
if [[ ! "${DEVELOPERS[$DEVELOPER_NAME]+exists}" ]]; then
    echo -e "${RED}❌ Error: Unknown developer '$DEVELOPER_NAME'${NC}"
    usage
fi

# 개발자 번호 및 포트 계산
DEVELOPER_ID=${DEVELOPERS[$DEVELOPER_NAME]}
REDIS_PORT=$((6378 + DEVELOPER_ID))  # 6379, 6380, 6381, ...
KIND_API_PORT=$((6442 + DEVELOPER_ID))  # 6443, 6444, 6445, ...

# 영문명 생성 (파일명용)
declare -A ENG_NAMES=(
    ["한진우"]="jinwoo"
    ["장준영"]="junyoung" 
    ["고민지"]="minji"
    ["이지윤"]="jiyoon"
    ["김보아"]="boa"
    ["유호준"]="hojun"
)

ENG_NAME=${ENG_NAMES[$DEVELOPER_NAME]}
ENV_FILE=".env.${ENG_NAME}.local"

echo -e "${GREEN}🎯 Setting up environment for: ${DEVELOPER_NAME} (${ENG_NAME})${NC}"
echo -e "${BLUE}📊 Assigned Resources:${NC}"
echo "  - Redis Port: ${REDIS_PORT}"
echo "  - Kind Cluster: ottoscaler-${ENG_NAME}"
echo "  - Environment File: ${ENV_FILE}"
echo ""

# .env 파일 생성
echo -e "${YELLOW}📝 Creating environment file: ${ENV_FILE}${NC}"

cat > "$ENV_FILE" << EOF
# Ottoscaler 환경 설정 - ${DEVELOPER_NAME} (${ENG_NAME})
# 이 파일은 ${DEVELOPER_NAME}의 개발 환경을 위해 자동 생성되었습니다
# 
# Redis 설정 - 개발자별 전용 Redis 인스턴스
REDIS_HOST=localhost
REDIS_PORT=${REDIS_PORT}
REDIS_PASSWORD=
REDIS_DB=0
REDIS_STREAM=otto:scale:events
REDIS_CONSUMER_GROUP=ottoscaler-${ENG_NAME}
REDIS_CONSUMER=ottoscaler-${ENG_NAME}-1

# Kubernetes 설정 - 개발자별 네임스페이스
NAMESPACE=${ENG_NAME}-dev

# Worker Pod 설정  
OTTO_AGENT_IMAGE=busybox:latest

# 개발자 정보
DEVELOPER_NAME=${DEVELOPER_NAME}
DEVELOPER_ENG_NAME=${ENG_NAME}
DEVELOPER_ID=${DEVELOPER_ID}

# Kind 클러스터 설정
KIND_CLUSTER_NAME=ottoscaler-${ENG_NAME}

# 환경별 설정 예시:
#
# ${DEVELOPER_NAME}의 전용 Redis:
# REDIS_HOST=localhost
# REDIS_PORT=${REDIS_PORT}
# 
# 다른 개발자와 격리된 환경:
# KIND_CLUSTER_NAME=ottoscaler-${ENG_NAME}
# NAMESPACE=${ENG_NAME}-dev
#
# 프로덕션 Redis (인증 사용 시):
# REDIS_HOST=redis.production.com
# REDIS_PASSWORD=your-redis-password
EOF

echo -e "${GREEN}✅ Environment file created: ${ENV_FILE}${NC}"

# 개발자별 Redis 컨테이너 이름 (otto-handler와 공유)
REDIS_CONTAINER_NAME="redis-${ENG_NAME}"

echo -e "${YELLOW}🗄️ Setting up Redis container: ${REDIS_CONTAINER_NAME}${NC}"
echo -e "${BLUE}   → otto-handler와 공유하여 사용합니다${NC}"

# 실행 중인 Redis 컨테이너 확인
if docker ps --filter "name=${REDIS_CONTAINER_NAME}" --filter "status=running" -q | grep -q .; then
    echo -e "${GREEN}✅ Redis container is already running: ${REDIS_CONTAINER_NAME} (port: ${REDIS_PORT})${NC}"
    echo -e "${BLUE}   → otto-handler에서 이미 생성된 컨테이너를 재사용합니다${NC}"
# 중지된 Redis 컨테이너 확인 및 재시작
elif docker ps -a --filter "name=${REDIS_CONTAINER_NAME}" -q | grep -q .; then
    echo -e "${YELLOW}⚠️  Redis container exists but is stopped. Restarting...${NC}"
    docker start "${REDIS_CONTAINER_NAME}"
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✅ Redis container restarted: ${REDIS_CONTAINER_NAME} (port: ${REDIS_PORT})${NC}"
    else
        echo -e "${RED}❌ Failed to restart Redis container. Removing and recreating...${NC}"
        docker rm -f "${REDIS_CONTAINER_NAME}"
        docker run -d --name ${REDIS_CONTAINER_NAME} \
            -p ${REDIS_PORT}:6379 \
            redis:7-alpine redis-server --appendonly yes
        echo -e "${GREEN}✅ Redis container created: ${REDIS_CONTAINER_NAME} (port: ${REDIS_PORT})${NC}"
    fi
# Redis 컨테이너가 없는 경우 새로 생성
else
    echo -e "${YELLOW}🆕 Creating Redis container '${REDIS_CONTAINER_NAME}' on port ${REDIS_PORT}...${NC}"
    echo -e "${BLUE}   → otto-handler에서도 이 컨테이너를 사용할 수 있습니다${NC}"
    docker run -d --name ${REDIS_CONTAINER_NAME} \
        -p ${REDIS_PORT}:6379 \
        redis:7-alpine redis-server --appendonly yes

    echo -e "${GREEN}✅ Redis container started: ${REDIS_CONTAINER_NAME} (port: ${REDIS_PORT})${NC}"
fi

# Kind 클러스터 설정
CLUSTER_NAME="ottoscaler-${ENG_NAME}"
echo -e "${YELLOW}☸️ Setting up Kind cluster: ${CLUSTER_NAME}${NC}"

# 기존 클러스터 확인
if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
    echo -e "${BLUE}✅ Kind cluster already exists: ${CLUSTER_NAME}${NC}"
else
    echo -e "${YELLOW}🆕 Creating Kind cluster: ${CLUSTER_NAME}${NC}"
    kind create cluster --name ${CLUSTER_NAME}
    echo -e "${GREEN}✅ Kind cluster created: ${CLUSTER_NAME}${NC}"
fi

# kubeconfig 컨텍스트 설정
kubectl config use-context kind-${CLUSTER_NAME}

# 네임스페이스 생성
NAMESPACE="${ENG_NAME}-dev"
echo -e "${YELLOW}📦 Setting up namespace: ${NAMESPACE}${NC}"
kubectl create namespace ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -

# RBAC 설정 적용 (개발자별 ServiceAccount 생성)
echo -e "${YELLOW}🔐 Creating ServiceAccount and RBAC...${NC}"
kubectl create serviceaccount ottoscaler -n ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -

# ClusterRole과 ClusterRoleBinding은 클러스터 전체에 한 번만 적용
kubectl apply -f k8s/rbac.yaml || echo -e "${BLUE}ℹ️ RBAC already exists${NC}"

# 개발자별 ClusterRoleBinding 생성
cat << EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ottoscaler-binding-${ENG_NAME}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: ottoscaler-role
subjects:
- kind: ServiceAccount
  name: ottoscaler
  namespace: ${NAMESPACE}
EOF

echo ""
echo -e "${GREEN}🎉 Setup completed for ${DEVELOPER_NAME}!${NC}"
echo ""
echo -e "${BLUE}📋 Next Steps:${NC}"
echo "  1. Load environment: source ${ENV_FILE}"
echo "  2. Test Redis: redis-cli -h localhost -p ${REDIS_PORT} ping"
echo "  3. Run ottoscaler: go run ./cmd/app"
echo "  4. Send test event: go run ./cmd/test-event"
echo ""
echo -e "${BLUE}📊 Your Resources:${NC}"
echo "  - Environment File: ${ENV_FILE}"
echo "  - Redis Container: ${REDIS_CONTAINER_NAME} (port: ${REDIS_PORT}) [otto-handler와 공유]"
echo "  - Kind Cluster: ${CLUSTER_NAME}"
echo "  - Namespace: ${NAMESPACE}"
echo ""
echo -e "${YELLOW}💡 Tip: Add 'source ${ENV_FILE}' to your ~/.bashrc for automatic loading${NC}"