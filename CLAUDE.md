# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 프로젝트 개요

CareerHub API Composer는 Go로 작성된 API 조합(Composition) 마이크로서비스이다. 3개의 업스트림 gRPC 서비스(채용공고, 사용자정보, 리뷰)를 집계하여 통합 REST API를 제공하는 API Gateway 역할을 한다. 자체 데이터베이스는 사용하지 않는다.

**spec-first다** — REST 계약의 진실은 `spec/api-composer/openapi.yaml`이고(디렉터리 경로 = 공표 태그 prefix), 서버 인터페이스·모델·라우팅 배선은 oapi-codegen으로 생성한다(`careerhub/apicomposer/api/api.gen.go`). 엔드포인트를 추가·변경할 때는 명세를 먼저 고치고 `make openapi`로 재생성한다 — 생성 코드를 손으로 수정하지 않는다.

## 빌드 및 실행

```bash
make build          # 바이너리 빌드 (./myapp)
make run            # 빌드 후 실행 (test.env 환경변수 사용)
make test           # 전체 테스트 실행 (-p 1, timeout 60s)
make openapi        # spec/api-composer/openapi.yaml → api.gen.go 재생성 (oapi-codegen)
make proto          # .proto 파일에서 gRPC 코드 재생성
make image_build    # Docker 이미지 빌드 (careerhub-api-composer:latest)
```

단일 테스트 실행:
```bash
go test -timeout 60s ./test/controller/...
```

테스트는 fake 기반이라 실 업스트림이 필요 없다 — `test.env`가 없으면 `make test`가 더미 기본값으로 돈다(CI 포함). `test.env`는 로컬 실행(`make run`)용이다.

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
| `OTEL_EXPORTER_OTLP_ENDPOINT` | (관측) OTLP gRPC 엔드포인트 — 트레이스 수집 |
| `OTEL_SERVICE_NAME` · `OTEL_RESOURCE_ATTRIBUTES` | (관측) 리소스 속성 — OTel SDK가 자동 흡수 |

## 아키텍처

```
HTTP 요청 → Gorilla Mux Router
  → Middleware 체인: otelmux(server span) → TraceContextLoggerMW → SetClaimsMW(JWT) → 명세 검증
  → 생성 배선(api.gen.go) → Strict Controller → Service → gRPC Client → 업스트림 서비스
```

### 핵심 계층 구조 (`careerhub/apicomposer/`)

- **`api/`** — 명세의 생성물. `api.gen.go`(생성 모델·strict 서버 인터페이스·gorilla 배선·내장 명세) + `oapi-codegen.yaml`(생성 설정). **계약의 진실은 저장소 루트 `spec/api-composer/openapi.yaml`**이고, 재생성은 `make openapi`
- **`app/`** — 애플리케이션 진입점. gRPC 연결 생성, 서비스/컨트롤러 의존성 주입, 미들웨어 체인·라우터 설정
- **`controller/`** — strict 서버 구현. 4개 컨트롤러(JobPosting, ScrapJob, MatchJob, Review)가 생성 인터페이스(`StrictServerInterface`)를 나눠 구현하고, `strict.go`가 배선·명세 검증 미들웨어·권한 미들웨어·에러 응답을 담당
- **`dto/`** — 컨트롤러 경계의 요청/응답 변환 모델
- **`posting/`**, **`userinfo/`**, **`review/`** — 비즈니스 로직 서비스 계층. 각 서비스는 gRPC 클라이언트를 래핑
- **`*/restapi_grpc/`** — protoc으로 생성된 gRPC 클라이언트 코드 (직접 수정 금지, `make proto`로 재생성)
- **`common/domain/`** — 공유 도메인 모델 (JobPosting, Review, ScrapJob, MatchJob)
- **`common/query/`** — 쿼리 필터링 로직
- **`jwtresolver/`** — JWT 토큰 파싱 및 Claims 관리
- **`middleware/`** — JWT Claims 주입(SetClaimsMW), OTel trace/span ID 로그 부착(TraceContextLoggerMW), 미들웨어 발신 에러의 ErrorResponse 응답
- **`observability/`** — OTel 트레이서 초기화 (OTLP gRPC exporter)
- **`vars/`** — 환경변수 로딩

### 주요 패턴

- **spec-first 생성 배선**: 전 라우트가 명세 생성 배선으로 등록된다(ADR-0018 — 손 등록 없음). 명세와 생성 코드의 어긋남은 CI 신선도 게이트가 거부한다
- **에러 계약**: 에러 응답은 상태 코드 불문 전부 명세의 `ErrorResponse`(application/json)다 — 미들웨어 발신(401/403)·요청 검증(400)·핸들러 오류(500) 모두. `http.Error`(text/plain)를 쓰지 않는다
- **요청 검증**: 내장 명세(`GetSwagger`) 기반 검증 미들웨어가 파라미터·본문의 형식 위반을 핸들러 도달 전에 400으로 거부
- **비동기 데이터 보강**: `goutils/cchan/async.ExecAsync()`를 사용하여 gRPC 호출을 병렬 실행. 보강 데이터(리뷰, 스크랩) 로딩 실패 시에도 부분 응답을 반환한다
- **쿼리 인코딩**: 복잡한 검색 쿼리는 base64로 인코딩하여 요청 파라미터로 전달 (디코딩은 컨트롤러에서)
- **인증**: `Authorization: Bearer {token}` 헤더에서 JWT 파싱. Claims에 userId와 authorities 포함. 무효 토큰은 원인 불문 401 + `ErrorResponse` JSON (RFC 6750)
- **로그인 필수 검사**: 명세의 `security` 선언이 원천 — 명세 검증 미들웨어의 `AuthenticationFunc`가 claims 존재를 확인. `{}` 대안 선언 연산은 비로그인 허용
- **권한 검사**: 생성 배선의 `requireAuthorityMW`(operationID 단위). 현재 MatchJob만 권한 필요 (`AUTHORITY_MATCH_JOB`)
- **분산 트레이싱**: otelmux가 incoming `traceparent`에서 server span을 시작하고, otelgrpc StatsHandler가 업스트림 gRPC 호출에 W3C TraceContext를 전파한다. 로그에는 OTel trace_id/span_id가 자동 부착된다

### 라우터 구조

- `/job_postings/**` — 채용공고 조회, 카테고리/스킬/회사 목록
- `/my/scrap-job/**` — 사용자 스크랩(북마크) 관리
- `/my/match-job/**` — 사용자 매칭 조건 관리 (권한 필요)
- `/company-review/**` — 회사 리뷰 조회

경로·연산의 정본은 `spec/api-composer/openapi.yaml`이다.

## CI/CD

- **머지 게이트** (`go-test.yaml`, PR 및 `prod`·`dev` push): ① 명세 신선도(`make openapi` 재생성 후 diff 없음) ② 빌드 ③ 전체 테스트 — 순차 실행, 앞이 실패하면 뒤는 돌지 않는다
- **이미지 빌드** (`create-tag.yaml`): `v*` 태그 push 시 GitHub Actions로 Docker 이미지를 빌드하여 AWS ECR(ap-northeast-2)에 푸시한다. main/prod push로는 빌드되지 않는다

## 의존성

- **oapi-codegen** — 명세 → Go 생성 (`go tool`로 실행, models·gorilla-server·strict-server·embedded-spec)
- **kin-openapi / nethttp-middleware** — 내장 명세 기반 요청 검증
- **Gorilla Mux/Handlers** — HTTP 라우팅 및 CORS
- **golang-jwt/jwt/v5** — JWT 처리
- **OpenTelemetry** (otel SDK, otelmux, otelgrpc, OTLP gRPC exporter) — 분산 트레이싱
- **google.golang.org/grpc** — gRPC 클라이언트
- **github.com/jae2274/goutils** — 로깅(llog), 에러 핸들링(terr), 비동기(cchan) 유틸리티
- **testify** — 테스트 assertion


## 문서 파일명 규칙

`docs/` 디렉토리의 문서 파일명은 `YYMMDDHHmm_제목.md` 형식으로 작성합니다. prefix는 문서 최초 생성 시각(KST)을 나타내며, 이를 통해 파일명 정렬만으로 생성 시간순 정렬이 가능합니다.

```
예시: 2603251254_refactoring-plan.md  (2026년 3월 25일 12시 54분 생성)
```

## 문서 작성 포맷

`docs/` 디렉토리에 계획서·분석서 등 문서를 작성할 때 아래 헤더를 반드시 포함합니다:

```markdown
# 문서 제목

**작성일**: YYYY-MM-DD HH:MM (KST)
**최종 수정일**: YYYY-MM-DD HH:MM (KST)
**대상 서비스**: 해당 문서가 다루는 서비스 목록 (쉼표 구분)

---
```

- 날짜와 시간은 한국 표준시(KST, UTC+9) 기준으로 기록합니다.
- 문서를 수정할 때마다 **최종 수정일**을 갱신합니다.
