package controller

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 합의 선행 머지(스펙+stub) 관행의 미구현 신호 계약(ADR-0019, ops-workflow#30):
// 공표됐으나 미구현인 연산은 생성 라우팅을 그대로 타고 501 + ErrorResponse로 응답한다.
// GetContractVersion이 구현되면 이 테스트는 성공 계약 테스트로 대체된다 —
// 그 대체가 곧 "구현 완료 확인"이다(README '합의 선행 머지' 절).
func TestMetaStrict_ContractVersionStub(t *testing.T) {
	f := newFakes()
	server, _ := newStrictTestServer(t, f)

	resp, body := doRequest(t, http.MethodGet, server.URL+"/api/contract/version", "", nil)

	require.Equal(t, http.StatusNotImplemented, resp.StatusCode)
	// 미구현 신호도 에러 계약을 지킨다: text/plain이 아니라 구조화 JSON(ErrorResponse)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	var errResp api.ErrorResponse
	require.NoError(t, json.Unmarshal(body, &errResp), "body: %s", body)
	assert.Equal(t, "Not Implemented", errResp.Message)
}
