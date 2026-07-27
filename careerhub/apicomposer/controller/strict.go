package controller

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"
)

// NewStrictHTTPHandler는 strict 구현을 생성 배선(HandlerWithOptions)으로 라우터에 얹는다.
// 에러 계약: 생성 코드가 잡는 오류(파라미터 바인딩 실패 등)도 핸들러 밖 오류도
// 전부 명세의 ErrorResponse(application/json)로 나간다 — 기본 핸들러(http.Error, text/plain)를 쓰지 않는다.
//
// 지금은 테스트만 이 함수를 쓴다. 전 컨트롤러 구현이 끝나는 일괄 전환 시점에
// app.go가 손 등록(RegisterRoutes) 대신 이 함수를 쓴다 (ops-workflow#14 코멘트의 사람 결정).
func NewStrictHTTPHandler(ssi api.StrictServerInterface, baseRouter *mux.Router) http.Handler {
	si := api.NewStrictHandlerWithOptions(ssi, nil, api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  writeErrorResponse(http.StatusBadRequest),
		ResponseErrorHandlerFunc: writeErrorResponse(http.StatusInternalServerError),
	})
	return api.HandlerWithOptions(si, api.GorillaServerOptions{
		BaseRouter:       baseRouter,
		ErrorHandlerFunc: writeErrorResponse(http.StatusBadRequest),
	})
}

func writeErrorResponse(status int) func(w http.ResponseWriter, r *http.Request, err error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(api.ErrorResponse{Message: err.Error()})
	}
}
