# Ottoscaler Development Environment
# 크로스 플랫폼 개발 환경을 위한 Docker 이미지
# 모든 개발 도구가 포함된 통합 환경

FROM golang:1.24-alpine

# 메타데이터
LABEL maintainer="Team-5-CodeCat"
LABEL description="Ottoscaler Development Environment"
LABEL version="1.0.0"

# 작업 디렉토리 설정
WORKDIR /workspace

# 시스템 패키지 업데이트 및 기본 도구 설치
RUN apk update && apk add --no-cache \
    # 기본 도구
    bash \
    git \
    curl \
    wget \
    make \
    # 빌드 도구
    build-base \
    # Protocol Buffers
    protobuf \
    protobuf-dev \
    # 네트워킹 도구 (Redis 연결 테스트용)
    redis \
    # 기타 유틸리티
    ca-certificates \
    tzdata \
    # 텍스트 에디터 (컨테이너 내 편집용)
    nano \
    vim

# Go 개발 도구 설치
RUN echo "Installing Go development tools..." && \
    # Protocol Buffer Go 플러그인들
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && \
    # 기타 유용한 Go 도구들
    go install golang.org/x/tools/cmd/goimports@latest && \
    go install golang.org/x/tools/cmd/godoc@latest

# golangci-lint 별도 설치 (바이너리 다운로드 방식)
RUN echo "Installing golangci-lint via binary..." && \
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin latest

# kubectl 설치 (Kubernetes 클러스터 작업용)
RUN echo "Installing kubectl..." && \
    curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    chmod +x kubectl && \
    mv kubectl /usr/local/bin/

# 환경 변수 설정
ENV GO111MODULE=on
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

# Go 모듈 캐시를 위한 디렉토리 생성
RUN mkdir -p /go/pkg/mod

# 개발 환경 설정
ENV PS1='🐹 ottoscaler-dev:\w$ '
ENV TERM=xterm-256color

# 헬스체크 스크립트 생성
RUN echo '#!/bin/bash\necho "Development environment is ready 🚀"\ngo version\nprotoc --version\ngolangci-lint version' > /usr/local/bin/health-check && \
    chmod +x /usr/local/bin/health-check

# Starship (크로스 쉘 프롬프트) 설치 및 설정
RUN echo "Installing Starship..." && \
    curl -sS https://starship.rs/install.sh | sh -s -- -y && \
    echo 'eval "$(starship init bash)"' >> /root/.bashrc

# Starship 개발 친화적 설정
RUN mkdir -p /root/.config && \
    cat > /root/.config/starship.toml << 'EOF'
# Starship Configuration for Ottoscaler Development

[character]
success_symbol = "[🐹](bold green)"
error_symbol = "[🐹](bold red)"

[directory]
truncation_length = 3
format = "[$path]($style)[$read_only]($read_only_style) "

[git_branch]
format = "[$symbol$branch]($style) "
symbol = "🌿 "

[git_status]
format = '([\[$all_status$ahead_behind\]]($style) )'

[golang]
format = "[$symbol($version )]($style)"
symbol = "🐹 "

[docker_context]
format = "[$symbol$context]($style) "
symbol = "🐳 "

[kubernetes]
format = '[$symbol$context( \($namespace\))]($style) '
symbol = "☸️ "
disabled = false

[redis]
format = "[$symbol$version]($style) "
symbol = "🗄️ "

[cmd_duration]
format = " took [$duration]($style)"

[package]
disabled = true

[aws]
disabled = true
EOF

# 개발 편의를 위한 alias 설정
RUN echo 'alias ll="ls -la"' >> /root/.bashrc && \
    echo 'alias gp="git pull"' >> /root/.bashrc && \
    echo 'alias gs="git status"' >> /root/.bashrc && \
    echo 'alias gr="go run"' >> /root/.bashrc && \
    echo 'alias gt="go test"' >> /root/.bashrc && \
    echo 'alias gb="go build"' >> /root/.bashrc && \
    echo 'export HISTSIZE=1000' >> /root/.bashrc && \
    echo 'export HISTFILESIZE=2000' >> /root/.bashrc

# 포트 노출 (향후 개발 서버용)
EXPOSE 8080 9090 50051

# 컨테이너 시작 시 실행할 명령어
# bash를 유지하여 개발자가 접속할 수 있도록 함
CMD ["/bin/bash"]

# 헬스체크 (컨테이너 상태 확인용)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD /usr/local/bin/health-check || exit 1