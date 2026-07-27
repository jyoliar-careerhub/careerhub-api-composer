# careerhub-api-composer

## REST API 명세 (OpenAPI)

REST 계약의 명세는 **`careerhub/apicomposer/api/openapi.yaml`** 에 있다 (ADR-0018: spec-first).

- **생성물**: 같은 디렉터리의 `api.gen.go` — 명세에서 생성한 Go 모델·gorilla/mux 서버 인터페이스(strict). proto 패턴(`.proto` 옆 `.pb.go` 커밋)과 같이 명세와 나란히 커밋한다.
- **재생성**: 명세를 수정했다면 아래 한 번으로 생성물을 갱신한다.

  ```bash
  make openapi
  ```

  도구는 [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) v2 — `go.mod`의 `tool` 지시자로 버전이 고정되어 있어 별도 설치가 필요 없다. 생성 설정은 `careerhub/apicomposer/api/oapi-codegen.yaml`.
- **CI 게이트**: `.github/workflows/openapi-freshness.yaml`이 PR·push마다 재생성 후 diff를 검사해 명세·생성물 불일치를 거부한다.

> **주의**: 전 컨트롤러가 생성 인터페이스로 구속되기 전까지(Epic jyoliar-lab/ops-workflow#13) 이 명세는 계약의 진실이 아니다. 계약을 바꿀 때는 명세를 먼저 수정하고(`openapi.yaml` → 리뷰), `make openapi`로 생성물을 갱신한 뒤 구현이 따라온다.
