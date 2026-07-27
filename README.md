# careerhub-api-composer

## REST API 명세 (OpenAPI)

REST 계약의 명세는 **`careerhub/apicomposer/api/`** 에 있다 (ADR-0018: spec-first). 루트 문서 `openapi.yaml`이 컨트롤러 경계대로 나뉜 경로 절편을 `$ref`로 조립한다:

| 파일 | 내용 |
|---|---|
| `openapi.yaml` | 루트 — info·tags·보안 스킴·경로 조립($ref) |
| `jobposting.yaml` | 채용공고 조회 6개 (`controller/jobposting_controller.go`) |
| `scrapjob.yaml` | 스크랩 관리 7개 (`controller/scrapJob_controller.go`) |
| `matchjob.yaml` | 매칭 조건 5개 (`controller/matchJob_controller.go`) |
| `review.yaml` | 회사 리뷰 1개 (`controller/review_controller.go`) |
| `components.yaml` | 공유 스키마(데이터 모델)의 단일 정의처 |

- **생성물**: 같은 디렉터리의 `models.gen.go`(공유 스키마 모델) + `api.gen.go`(gorilla/mux 서버·strict 인터페이스). proto 패턴(`.proto` 옆 `.pb.go` 커밋)과 같이 명세와 나란히 커밋한다.
- **재생성**: 명세를 수정했다면 아래 한 번으로 생성물을 갱신한다.

  ```bash
  make openapi
  ```

  도구는 [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) v2 — `go.mod`의 `tool` 지시자로 버전이 고정되어 있어 별도 설치가 필요 없다. 생성 설정은 `careerhub/apicomposer/api/oapi-codegen.yaml`.
- **CI 게이트**: `.github/workflows/openapi-freshness.yaml`이 PR·push마다 재생성 후 diff를 검사해 명세·생성물 불일치를 거부한다.

> **주의**: 전 컨트롤러가 생성 인터페이스로 구속되기 전까지(Epic jyoliar-lab/ops-workflow#13) 이 명세는 계약의 진실이 아니다. 계약을 바꿀 때는 명세를 먼저 수정하고(`openapi.yaml` → 리뷰), `make openapi`로 생성물을 갱신한 뒤 구현이 따라온다.
