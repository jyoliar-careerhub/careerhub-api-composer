package controller

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/gorilla/mux"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/common/user_authority"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/middleware"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"
)

// StrictServer는 컨트롤러별 strict 구현을 임베딩으로 조립해 전체 인터페이스를 만족시킨다.
// 아래 단언이 배선의 핵심 보증이다: 명세의 19개 연산 전부가 정확한 시그니처로 구현되어야
// 컴파일되고, 연산을 명세에 추가하고 구현을 빠뜨려도, 두 컨트롤러가 같은 연산을 중복
// 구현해도(모호성 → 메서드 집합 제외) 여기서 컴파일이 깨진다.
type StrictServer struct {
	*JobPostingStrictController
	*ScrapJobStrictController
	*MatchJobStrictController
	*ReviewStrictController
	*MetaStrictController
}

var _ api.StrictServerInterface = (*StrictServer)(nil)

func NewStrictServer(
	jobPosting *JobPostingStrictController,
	scrapJob *ScrapJobStrictController,
	matchJob *MatchJobStrictController,
	review *ReviewStrictController,
	meta *MetaStrictController,
) *StrictServer {
	return &StrictServer{
		JobPostingStrictController: jobPosting,
		ScrapJobStrictController:   scrapJob,
		MatchJobStrictController:   matchJob,
		ReviewStrictController:     review,
		MetaStrictController:       meta,
	}
}

// NewStrictHTTPHandler는 strict 구현을 생성 배선(HandlerWithOptions)으로 라우터에 얹는다.
// 에러 계약: 생성 코드가 잡는 오류(파라미터 바인딩·본문 디코드 실패 등)도 핸들러 밖 오류도
// 전부 명세의 ErrorResponse(application/json)로 나간다 — 기본 핸들러(http.Error, text/plain)를 쓰지 않는다.
// 관문 둘이 operationID 단위로 걸린다 — 401/403 모두 ErrorResponse(JSON)다:
//   - 동의 게이트: 보호 연산 전부(명세에서 파생)에 필수 약관 동의 충족을 요구한다(ops-workflow#54)
//   - 권한: matchJob 연산에 AUTHORITY_MATCH_JOB을 요구한다
func NewStrictHTTPHandler(ssi api.StrictServerInterface, baseRouter *mux.Router) (http.Handler, error) {
	swagger, err := api.GetSpec()
	if err != nil {
		return nil, err
	}

	si := api.NewStrictHandlerWithOptions(ssi,
		[]api.StrictMiddlewareFunc{
			requireAgreementsMW(protectedOperations(swagger)),
			requireAuthorityMW(user_authority.AuthorityMatchJob, matchJobOperations),
		},
		api.StrictHTTPServerOptions{
			RequestErrorHandlerFunc:  writeErrorResponse(http.StatusBadRequest),
			ResponseErrorHandlerFunc: writeErrorResponse(http.StatusInternalServerError),
		})
	return api.HandlerWithOptions(si, api.GorillaServerOptions{
		BaseRouter:       baseRouter,
		ErrorHandlerFunc: writeErrorResponse(http.StatusBadRequest),
	}), nil
}

// NewSpecValidationMW는 들어오는 요청을 내장 명세(GetSpec)와 대조하는 미들웨어를 만든다.
// 필수 누락·타입 불일치·minimum/maximum 위반이 핸들러 도달 전에 ErrorResponse 400으로
// 거부된다 — 검증 규칙의 원천이 명세 하나로 수렴한다(strict 구현에는 명세로 표현 불가한
// 검증만 남는다). rootPath는 배포 prefix(ROOT_PATH) — 명세 경로와 대조 전에 잘라낸다.
// 로그인 필수 검사도 여기서 한다: 명세의 security 선언(bearerAuth)이 원천이고,
// openapi3filter가 요구 연산에서만 AuthenticationFunc를 호출한다 — SetClaimsMW가 앞서
// 주입한 claims의 존재만 확인하면 되고, `{}` 대안이 선언된 공개 연산은 자동 통과한다.
// 새 보호 엔드포인트는 명세에 security 선언만 하면 자동 적용된다.
func NewSpecValidationMW(rootPath string) (mux.MiddlewareFunc, error) {
	swagger, err := api.GetSpec()
	if err != nil {
		return nil, err
	}

	validator := nethttpmiddleware.OapiRequestValidatorWithOptions(swagger, &nethttpmiddleware.Options{
		DoNotValidateServers: true,
		Prefix:               rootPath,
		Options: openapi3filter.Options{
			AuthenticationFunc: requireLoginAuthenticationFunc,
		},
		ErrorHandler: func(w http.ResponseWriter, message string, statusCode int) {
			// 401은 원인 미노출 일반 문구로 고정한다 — openapi3filter의
			// SecurityRequirementsError 문자열을 그대로 내보내지 않는다.
			if statusCode == http.StatusUnauthorized {
				message = middleware.MessageUnauthorized
			}
			middleware.WriteError(w, statusCode, message)
		},
	})
	return mux.MiddlewareFunc(validator), nil
}

// requireLoginAuthenticationFunc — 명세가 bearerAuth를 요구하는 연산의 로그인 검사.
// 토큰의 유효성은 SetClaimsMW가 이미 판정했으므로(무효면 여기 도달 전에 401),
// ctx에 claims가 있는지만 본다.
func requireLoginAuthenticationFunc(ctx context.Context, input *openapi3filter.AuthenticationInput) error {
	if _, ok := middleware.GetClaims(ctx); !ok {
		return errors.New(middleware.MessageUnauthorized)
	}
	return nil
}

const messageInternalServerError = "Internal Server Error"
const messageForbidden = "Forbidden"

// matchJobOperations — AUTHORITY_MATCH_JOB이 필요한 연산(operationId의 생성명).
// 명세의 match-job 태그 그룹과 일치해야 한다.
var matchJobOperations = map[string]bool{
	"FindMatchJob":            true,
	"AddMatchJobCondition":    true,
	"UpdateMatchJobCondition": true,
	"DeleteMatchJobCondition": true,
	"UpdateAgreeToMail":       true,
}

// protectedOperations — 명세에서 파생한 보호 연산의 집합(생성 operationID 이름).
// "bearerAuth만 요구하고 비로그인 대안(`{}`)이 없는 연산"이 곧 보호 기능이다 —
// 손으로 두 번째 목록을 유지하면 명세의 security 선언과 조용히 갈라지므로 파생한다.
func protectedOperations(swagger *openapi3.T) map[string]bool {
	operations := map[string]bool{}
	for _, pathItem := range swagger.Paths.Map() {
		for _, operation := range pathItem.Operations() {
			if operation.Security == nil || len(*operation.Security) == 0 {
				continue
			}
			gated := true
			for _, requirement := range *operation.Security {
				if len(requirement) == 0 { // `{}` 대안 — 비로그인 허용 연산
					gated = false
				}
			}
			if gated {
				// strict 미들웨어가 받는 operationID는 생성된 메서드 이름(첫 글자 대문자)이다.
				operations[strings.ToUpper(operation.OperationID[:1])+operation.OperationID[1:]] = true
			}
		}
	}
	return operations
}

// requireAgreementsMW — 보호 연산의 필수 약관 동의 게이트(ops-workflow#54).
// 판정 입력은 클레임(`careerhub:agreements_fulfilled`)뿐이다 — 요청 경로에서 동의 상태를
// 조회하지 않는다(ADR-0031). 프론트의 온보딩 유도는 UX 게이트고, 집행은 이 403이 진다.
// 클레임은 발급 시점 스냅샷이라 동의 완료의 발효는 토큰 갱신에 묶인다 — 그 갱신 트리거는
// 앱(careerhub-web) 흐름의 몫이다.
func requireAgreementsMW(operationIDs map[string]bool) api.StrictMiddlewareFunc {
	return func(f api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
		if !operationIDs[operationID] {
			return f
		}
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
			claims, ok := middleware.GetClaims(ctx)
			if !ok {
				middleware.WriteError(w, http.StatusUnauthorized, middleware.MessageUnauthorized)
				return nil, nil
			}
			if !claims.AgreementsFulfilled {
				middleware.WriteError(w, http.StatusForbidden, messageForbidden)
				return nil, nil
			}
			return f(ctx, w, r, request)
		}
	}
}

// requireAuthorityMW는 지정 연산에만 권한을 요구하는 strict 미들웨어다.
// 401/403 모두 ErrorResponse(JSON)다. 401 분기는 방어적이다 — 로그인 필수 검사는
// 명세 security 기반의 AuthenticationFunc가 먼저 끊는다.
func requireAuthorityMW(authority string, operationIDs map[string]bool) api.StrictMiddlewareFunc {
	return func(f api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
		if !operationIDs[operationID] {
			return f
		}
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
			claims, ok := middleware.GetClaims(ctx)
			if !ok {
				middleware.WriteError(w, http.StatusUnauthorized, middleware.MessageUnauthorized)
				return nil, nil
			}
			if !claims.HasAuthority(authority) {
				middleware.WriteError(w, http.StatusForbidden, messageForbidden)
				return nil, nil
			}
			return f(ctx, w, r, request)
		}
	}
}

func writeErrorResponse(status int) func(w http.ResponseWriter, r *http.Request, err error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		middleware.WriteError(w, status, err.Error())
	}
}
