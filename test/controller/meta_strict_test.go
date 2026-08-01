package controller

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetaStrict_ContractVersion(t *testing.T) {
	f := newFakes()
	server, _ := newStrictTestServer(t, f)

	resp, body := doRequest(t, http.MethodGet, server.URL+"/api/contract/version", "", nil)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")

	var got api.ContractVersion
	require.NoError(t, json.Unmarshal(body, &got), "body: %s", body)

	// 내장 명세의 info.version과 일치해야 한다 — 이 값이 공표 태그
	// (spec/api-composer/vX.Y.Z)의 X.Y.Z이고, 소비자가 서버의 계약 판본을
	// 판별하는 근거다. 리터럴 대신 명세에서 읽어 비교하므로 버전이 올라가도
	// 이 테스트는 깨지지 않는다.
	swagger, err := api.GetSpec()
	require.NoError(t, err)
	assert.Equal(t, swagger.Info.Version, got.Version)
	assert.NotEmpty(t, got.Version)
}
