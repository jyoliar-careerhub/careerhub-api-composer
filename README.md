# careerhub-api-composer

## REST API 명세 (OpenAPI)

REST 계약의 명세는 **`spec/api-composer/openapi.yaml`** 에 있다 (ADR-0018: spec-first). 디렉터리 경로가 그대로 공표 태그의 prefix다(아래 '계약 공표 좌표'). 다른 포트의 provider 역할이 늘면 `spec/<이름>/`으로 나란히 분리된다.

- **생성물**: 같은 디렉터리의 `api.gen.go` — 명세에서 생성한 Go 모델·gorilla/mux 서버 인터페이스(strict). proto 패턴(`.proto` 옆 `.pb.go` 커밋)과 같이 명세와 나란히 커밋한다.
- **재생성**: 명세를 수정했다면 아래 한 번으로 생성물을 갱신한다.

  ```bash
  make openapi
  ```

  도구는 [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) v2 — `go.mod`의 `tool` 지시자로 버전이 고정되어 있어 별도 설치가 필요 없다. 생성 설정은 `careerhub/apicomposer/api/oapi-codegen.yaml`.
- **CI 게이트**: `.github/workflows/go-test.yaml`이 PR·push마다 재생성 후 diff를 검사해 명세·생성물 불일치를 거부한다(신선도 → 빌드 → 테스트 순차).

> **주의**: 전 컨트롤러가 생성 인터페이스로 구속되기 전까지(Epic jyoliar-lab/ops-workflow#13) 이 명세는 계약의 진실이 아니다. 계약을 바꿀 때는 명세를 먼저 수정하고(`openapi.yaml` → 리뷰), `make openapi`로 생성물을 갱신한 뒤 구현이 따라온다.

## 계약 공표 좌표 — `<계약 디렉터리>/vX.Y.Z` 태그 (ADR-0019·0020)

소비자(careerhub-web 등)는 가변 브랜치의 `openapi.yaml`이 아니라 **불변 태그에 핀한다**.

- **좌표 규약: 태그 prefix = 계약이 사는 디렉터리, 파일명은 고정 관례(`openapi.yaml`)** — 좌표에서 파일 위치가 파생된다. 예: 태그 `spec/api-composer/v0.2.0` → 그 태그의 `spec/api-composer/openapi.yaml`. 소비자에게 별도 경로 지식이 필요 없다(gRPC의 Go prefix 태그 규칙과 동형).
- `openapi.yaml`을 바꾼 머지(prod)마다 CI(`.github/workflows/spec-tag.yaml`)가 `info.version`을 읽어 **`spec/api-composer/v{version}` 태그를 자동 생성**한다. 손 태그는 없다.
- 따라서 **스펙 변경은 `info.version` 범프를 요구한다** — 같은 버전 태그가 이미 있으면 PR 검사(`check-version-bumped`)와 태그 생성이 실패한다.
- 버전은 **좌표일 뿐이다** — semver 의미(비호환=major 등)는 아직 부여하지 않는다(ADR-0021 논의 보류 중). 어느 버전에 올라탈지는 소비자가 자기 저장소의 핀으로 선택한다.
- 앱 빌드 태그(`v*` → `create-tag.yaml` → ECR)와는 무관하다 — 계약 디렉터리는 `v`로 시작하지 않으므로 `v*`에 매칭되지 않는다. 스펙 디렉터리를 옮기면 태그 prefix도 새 경로를 따른다(경로는 얼지 않는다 — `spec-tag.yaml`의 `SPEC_FILE`과 `paths` 두 곳을 함께 바꾼다).

## 합의 선행 머지 — 스펙+stub (ADR-0019)

계약 합의의 공표 지점은 **구현 완료가 아니라 스펙의 머지**다. 합의된 연산은 구현 전에 머지(공표)하고, provider의 구현과 소비자의 개발이 병렬로 진행된다. "공표됐으나 미구현"은 오류가 아니라 정상 상태다.

**무엇을 stub으로 머지할 수 있나** — 계약(연산의 형태)이 합의된 것 전부. 구현 일정과 무관하다. 머지 묶음은:

1. `openapi.yaml` 변경 — 신규 연산 + 해당 연산에 `'501'`(`NotImplemented`) 응답 선언 + `info.version` 범프
2. `make openapi` 재생성물 (`api.gen.go`)
3. strict 메서드 stub — `notImplemented501[...]` 반환. 미구현 신호도 에러 계약(`ErrorResponse` JSON)을 지킨다. 임베딩 조립(`controller/strict.go`)이 그대로 만족되므로 별도 배선은 없다
4. stub 응답 테스트(`test/controller/*_strict_test.go`) — 501 + `ErrorResponse` 단언

머지 게이트(신선도 → 빌드 → 테스트)는 그대로 적용된다. 첫 사례: `GET /contract/version` (`controller/meta_strict.go`).

**구현 완료는 어떻게 확인하는가** — 컴파일이 아니라 **테스트와 사람**이다(ADR-0019). stub을 실 구현으로 바꾸는 PR은:

- 해당 연산의 **동작 테스트**(생성 라우팅 경유)로 stub의 501 테스트를 **대체**한다 — 이 대체가 구현 완료의 기계 확인이다.
- 명세에서 해당 연산의 `'501'` 선언을 제거하고 `info.version`을 범프한다 — "이 버전부터 구현 서빙을 의도한다"는 계약 변경이므로 새 공표 좌표가 나간다. (배포가 따라오기 전까지 구버전 서버를 만날 수 있음은 소비자 몫의 전제다 — 신호 구분은 `GET /contract/version`으로.)
