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

# 개발자 목록
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

# 개발자 번호
DEVELOPER_ID=${DEVELOPERS[$DEVELOPER_NAME]}

# 영문명 생성 (파일명용)
declare -A ENG_NAMES=(
    ["한진우"]="hanjinwoo"
    ["장준영"]="jangjunyoung" 
    ["고민지"]="gominji"
    ["이지윤"]="leejiyun"
    ["김보아"]="kimboa"
    ["유호준"]="yoohojun"
)

ENG_NAME=${ENG_NAMES[$DEVELOPER_NAME]}
ENV_FILE=".env.${ENG_NAME}.local"

echo -e "${GREEN}🎯 ${DEVELOPER_NAME} (${ENG_NAME}) 개발 환경을 설정합니다${NC}"
echo -e "${BLUE}📊 할당된 리소스:${NC}"
echo "  - Kind 클러스터: ottoscaler-${ENG_NAME}"
echo "  - 네임스페이스: ${ENG_NAME}-dev"
echo "  - 환경 파일: ${ENV_FILE}"
echo ""

# .env 파일 생성
echo -e "${YELLOW}📝 환경 파일을 생성합니다: ${ENV_FILE}${NC}"

cat > "$ENV_FILE" << EOF
# Ottoscaler 환경 설정 - ${DEVELOPER_NAME} (${ENG_NAME})
# 이 파일은 ${DEVELOPER_NAME}의 개발 환경을 위해 자동 생성되었습니다

# gRPC 서버 설정
GRPC_PORT=9090
OTTO_HANDLER_HOST=otto-handler:8080

# Kubernetes 설정 - 개발자별 네임스페이스
NAMESPACE=${ENG_NAME}-dev

# Worker Pod 설정  
OTTO_AGENT_IMAGE=busybox:latest
WORKER_CPU_LIMIT=500m
WORKER_MEMORY_LIMIT=128Mi

# 로깅 설정
LOG_LEVEL=info

# 개발자 정보
DEVELOPER_NAME=${DEVELOPER_NAME}
DEVELOPER_ENG_NAME=${ENG_NAME}
DEVELOPER_ID=${DEVELOPER_ID}

# Kind 클러스터 설정
KIND_CLUSTER_NAME=ottoscaler-${ENG_NAME}
EOF

echo -e "${GREEN}✅ 환경 파일이 생성되었습니다: ${ENV_FILE}${NC}"

# Kind 클러스터 설정
CLUSTER_NAME="ottoscaler-${ENG_NAME}"
echo -e "${YELLOW}☸️ Kind 클러스터를 확인합니다: ${CLUSTER_NAME}${NC}"

# 기존 클러스터 확인
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    # 클러스터가 실제로 실행 중인지 확인
    if kubectl cluster-info --context kind-${CLUSTER_NAME} &>/dev/null; then
        echo -e "${GREEN}✅ Kind 클러스터가 이미 실행 중입니다: ${CLUSTER_NAME}${NC}"
        echo -e "${BLUE}   → 기존 실행 중인 클러스터를 사용합니다. 클러스터 생성을 건너뜁니다.${NC}"
    else
        echo -e "${YELLOW}⚠️  Kind 클러스터가 존재하지만 접근할 수 없습니다. 재생성합니다...${NC}"
        kind delete cluster --name ${CLUSTER_NAME}
        kind create cluster --name ${CLUSTER_NAME}
        echo -e "${GREEN}✅ Kind 클러스터가 재생성되었습니다: ${CLUSTER_NAME}${NC}"
    fi
else
    echo -e "${YELLOW}🆕 Kind 클러스터를 생성합니다: ${CLUSTER_NAME}${NC}"
    kind create cluster --name ${CLUSTER_NAME}
    echo -e "${GREEN}✅ Kind 클러스터가 생성되었습니다: ${CLUSTER_NAME}${NC}"
fi

# kubeconfig 컨텍스트 설정
kubectl config use-context kind-${CLUSTER_NAME}

# 네임스페이스 생성
NAMESPACE="${ENG_NAME}-dev"
echo -e "${YELLOW}📦 네임스페이스를 설정합니다: ${NAMESPACE}${NC}"
kubectl create namespace ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -

# RBAC 설정 적용 (개발자별 ServiceAccount 생성)
echo -e "${YELLOW}🔐 ServiceAccount와 RBAC를 생성합니다...${NC}"
kubectl create serviceaccount ottoscaler -n ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -

# ClusterRole과 ClusterRoleBinding은 클러스터 전체에 한 번만 적용
kubectl apply -f k8s/rbac.yaml || echo -e "${BLUE}ℹ️ RBAC가 이미 존재합니다${NC}"

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
echo -e "${GREEN}🎉 ${DEVELOPER_NAME}의 환경 설정이 완료되었습니다!${NC}"
echo ""
echo -e "${BLUE}📋 다음 단계:${NC}"
echo "  1. Docker 이미지 빌드: make build"
echo "  2. Kind 클러스터에 배포: make deploy"
echo "  3. 로그 확인: make logs"
echo "  4. 테스트: ./test-scaling -action scale-up -workers 3"
echo ""
echo -e "${BLUE}📊 할당된 리소스:${NC}"
echo "  - 환경 파일: ${ENV_FILE}"
echo "  - Kind 클러스터: ${CLUSTER_NAME}"
echo "  - 네임스페이스: ${NAMESPACE}"
echo ""
echo -e "${YELLOW}💡 팁: kubectl port-forward deployment/ottoscaler 9090:9090 으로 gRPC 서버에 접근할 수 있습니다${NC}"