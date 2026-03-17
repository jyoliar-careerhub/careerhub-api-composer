# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 프로젝트 개요

CareerHub API Composer는 Go로 작성된 API 조합(Composition) 마이크로서비스이다. 3개의 업스트림 gRPC 서비스(채용공고, 사용자정보, 리뷰)를 집계하여 통합 REST API를 제공하는 API Gateway 역할을 한다. 자체 데이터베이스는 사용하지 않는다.

## 빌드 및 실행

```bash
make build          # 바이너리 빌드 (./myapp)
make run            # 빌드 후 실행 (test.env 환경변수 사용)
make test           # 전체 테스트 실행 (-p 1, timeout 60s)
make proto          # .proto 파일에서 gRPC 코드 재생성
make image_build    # Docker 이미지 빌드 (careerhub-api-composer:latest)
```

단일 테스트 실행:
```bash
go test -timeout 60s ./test/controller/...
```

테스트 실행 시 환경변수(`POSTING_GRPC_ENDPOINT`, `API_PORT`, `SECRET_KEY`, `REVIEW_GRPC_ENDPOINT`)가 필요하다. `test.env` 파일에 정의되어 있다.

## 필수 환경변수

| 변수 | 설명 |
|---|---|
| `POSTING_GRPC_ENDPOINT` | 채용공고 gRPC 서비스 주소 |
| `USERINFO_GRPC_ENDPOINT` | 사용자정보 gRPC 서비스 주소 |
| `REVIEW_GRPC_ENDPOINT` | 리뷰 gRPC 서비스 주소 |
| `API_PORT` | HTTP 서버 포트 |
| `SECRET_KEY` | JWT 서명 키 |
| `ACCESS_CONTROL_ALLOW_ORIGIN` | (선택) CORS 허용 origin |
| `ROOT_PATH` | (선택) API 경로 prefix |

## 아키텍처

```
HTTP 요청 → Gorilla Mux Router → Middleware(TraceID, JWT) → Controller → Service → gRPC Client → 업스트림 서비스
```

### 핵심 계층 구조 (`careerhub/apicomposer/`)

- **`app/`** — 애플리케이션 진입점. gRPC 연결 생성, 서비스/컨트롤러 의존성 주입, 라우터 설정
- **`controller/`** — HTTP 핸들러. 4개 컨트롤러: JobPosting, ScrapJob, MatchJob, Review
- **`posting/`**, **`userinfo/`**, **`review/`** — 비즈니스 로직 서비스 계층. 각 서비스는 gRPC 클라이언트를 래핑
- **`*/restapi_grpc/`** — protoc으로 생성된 gRPC 클라이언트 코드 (직접 수정 금지, `make proto`로 재생성)
- **`common/domain/`** — 공유 도메인 모델 (JobPosting, Review, ScrapJob, MatchJob)
- **`common/query/`** — 쿼리 필터링 로직
- **`jwtresolver/`** — JWT 토큰 파싱 및 Claims 관리
- **`middleware/`** — 인증 미들웨어 (JWT Claims 주입, 권한 검사)
- **`vars/`** — 환경변수 로딩
- **`httputils/`** — HTTP 에러 핸들링 유틸리티

### 주요 패턴

- **비동기 데이터 보강**: `goutils/cchan/async.ExecAsync()`를 사용하여 gRPC 호출을 병렬 실행. 보강 데이터(리뷰, 스크랩) 로딩 실패 시에도 부분 응답을 반환한다.
- **쿼리 인코딩**: 복잡한 검색 쿼리는 base64로 인코딩하여 요청 파라미터로 전달
- **인증**: `Authorization: Bearer {token}` 헤더에서 JWT 파싱. Claims에 userId와 authorities 포함
- **권한 검사**: `middleware.CheckHasAuthority()`로 라우터 단위 권한 검사. 현재 MatchJob만 권한 필요 (`AUTHORITY_MATCH_JOB`)

### 라우터 구조

- `/job_postings/**` — 채용공고 조회, 카테고리/스킬/회사 목록
- `/my/scrap-job/**` — 사용자 스크랩(북마크) 관리
- `/my/match-job/**` — 사용자 매칭 조건 관리 (권한 필요)
- `/company-review/**` — 회사 리뷰 조회

## CI/CD

`v*` 태그 push 시 GitHub Actions로 Docker 이미지를 빌드하여 AWS ECR(ap-northeast-2)에 푸시한다.

## 의존성

- **Gorilla Mux/Handlers** — HTTP 라우팅 및 CORS
- **golang-jwt/jwt/v5** — JWT 처리
- **google.golang.org/grpc** — gRPC 클라이언트
- **github.com/jae2274/goutils** — 로깅(llog), 에러 핸들링(terr), 미들웨어(mw) 유틸리티
- **testify** — 테스트 assertion
