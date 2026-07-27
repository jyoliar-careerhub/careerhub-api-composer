package controller

import (
	"context"
	"encoding/json"
	"net/http"

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
}

var _ api.StrictServerInterface = (*StrictServer)(nil)

func NewStrictServer(
	jobPosting *JobPostingStrictController,
	scrapJob *ScrapJobStrictController,
	matchJob *MatchJobStrictController,
	review *ReviewStrictController,
) *StrictServer {
	return &StrictServer{
		JobPostingStrictController: jobPosting,
		ScrapJobStrictController:   scrapJob,
		MatchJobStrictController:   matchJob,
		ReviewStrictController:     review,
	}
}

// NewStrictHTTPHandler는 strict 구현을 생성 배선(HandlerWithOptions)으로 라우터에 얹는다.
// 에러 계약: 생성 코드가 잡는 오류(파라미터 바인딩·본문 디코드 실패 등)도 핸들러 밖 오류도
// 전부 명세의 ErrorResponse(application/json)로 나간다 — 기본 핸들러(http.Error, text/plain)를 쓰지 않는다.
// 권한: matchJob 연산에는 기존 CheckHasAuthority(라우터 단위)를 operationID 단위로 옮긴
// 권한 미들웨어가 걸린다 — 401/403 응답 형태(본문 없음)는 기존 미들웨어와 동일하다.
func NewStrictHTTPHandler(ssi api.StrictServerInterface, baseRouter *mux.Router) http.Handler {
	si := api.NewStrictHandlerWithOptions(ssi,
		[]api.StrictMiddlewareFunc{
			requireAuthorityMW(user_authority.AuthorityMatchJob, matchJobOperations),
		},
		api.StrictHTTPServerOptions{
			RequestErrorHandlerFunc:  writeErrorResponse(http.StatusBadRequest),
			ResponseErrorHandlerFunc: writeErrorResponse(http.StatusInternalServerError),
		})
	return api.HandlerWithOptions(si, api.GorillaServerOptions{
		BaseRouter:       baseRouter,
		ErrorHandlerFunc: writeErrorResponse(http.StatusBadRequest),
	})
}

// NewSpecValidationMW는 들어오는 요청을 내장 명세(GetSwagger)와 대조하는 미들웨어를 만든다.
// 필수 누락·타입 불일치·minimum/maximum 위반이 핸들러 도달 전에 ErrorResponse 400으로
// 거부된다 — 검증 규칙의 원천이 명세 하나로 수렴한다(strict 구현에는 명세로 표현 불가한
// 검증만 남는다). rootPath는 배포 prefix(ROOT_PATH) — 명세 경로와 대조 전에 잘라낸다.
// 인증·권한은 이 미들웨어의 관할이 아니다(기존 미들웨어 체인이 한다).
func NewSpecValidationMW(rootPath string) (mux.MiddlewareFunc, error) {
	swagger, err := api.GetSwagger()
	if err != nil {
		return nil, err
	}

	validator := nethttpmiddleware.OapiRequestValidatorWithOptions(swagger, &nethttpmiddleware.Options{
		DoNotValidateServers: true,
		Prefix:               rootPath,
		Options: openapi3filter.Options{
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
		},
		ErrorHandler: func(w http.ResponseWriter, message string, statusCode int) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			_ = json.NewEncoder(w).Encode(api.ErrorResponse{Message: message})
		},
	})
	return mux.MiddlewareFunc(validator), nil
}

const messageInternalServerError = "Internal Server Error"

// matchJobOperations — AUTHORITY_MATCH_JOB이 필요한 연산(operationId의 생성명).
// 명세의 match-job 태그 그룹과 일치해야 한다.
var matchJobOperations = map[string]bool{
	"FindMatchJob":            true,
	"AddMatchJobCondition":    true,
	"UpdateMatchJobCondition": true,
	"DeleteMatchJobCondition": true,
	"UpdateAgreeToMail":       true,
}

// requireAuthorityMW는 지정 연산에만 권한을 요구하는 strict 미들웨어다.
// 응답은 기존 CheckHasAuthority와 동일: 401/403 모두 본문 없음.
func requireAuthorityMW(authority string, operationIDs map[string]bool) api.StrictMiddlewareFunc {
	return func(f api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
		if !operationIDs[operationID] {
			return f
		}
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
			claims, ok := middleware.GetClaims(ctx)
			if !ok {
				w.WriteHeader(http.StatusUnauthorized)
				return nil, nil
			}
			if !claims.HasAuthority(authority) {
				w.WriteHeader(http.StatusForbidden)
				return nil, nil
			}
			return f(ctx, w, r, request)
		}
	}
}

func writeErrorResponse(status int) func(w http.ResponseWriter, r *http.Request, err error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(api.ErrorResponse{Message: err.Error()})
	}
}
