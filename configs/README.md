# /configs

설정 파일 템플릿 및 기본 설정 디렉터리입니다.

## 목적

이 디렉터리는 다양한 환경별 설정 파일들과 설정 템플릿을 관리하기 위해 예약되어 있습니다.

## 향후 구성 예시

```
configs/
├── development/
│   ├── redis.yaml      # 개발환경 Redis 설정
│   └── k8s.yaml        # 개발환경 K8s 설정
├── production/
│   ├── redis.yaml      # 운영환경 Redis 설정
│   └── k8s.yaml        # 운영환경 K8s 설정
├── templates/
│   ├── config.yaml.tmpl    # 설정 템플릿
│   └── env.template        # 환경변수 템플릿
└── README.md
```

## 사용 원칙

- **환경별 분리**: development, production, staging 등
- **템플릿 활용**: `confd`, `consul-template` 등과 연동
- **민감 정보 제외**: 비밀번호, API 키 등은 별도 시크릿 관리
- **버전 관리**: 설정 파일도 Git으로 추적

## 현재 상태

현재는 환경변수를 통한 설정 방식을 사용하고 있습니다:
- `REDIS_HOST`, `REDIS_PORT`
- `KUBERNETES_NAMESPACE`
- `OTTO_AGENT_IMAGE`

필요에 따라 YAML 기반 설정 파일로 확장할 예정입니다.