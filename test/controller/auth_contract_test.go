package controller

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/jwtresolver"
	postingGrpc "github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/posting/restapi_grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 미들웨어 발신 에러의 계약 테스트 (jyoliar-lab/ops-workflow#21):
// 무효 토큰은 원인 불문(만료·형식 불량·서명 불일치) 401 + ErrorResponse JSON이고,
// 원인은 본문에 노출하지 않는다. 로그인 필수 여부의 원천은 명세의 security 선언이다.

func expiredToken(t *testing.T) string {
	t.Helper()
	// access 토큰 수명 10분 — 1시간 전 발급이면 만료 상태다
	tokens, err := jwtresolver.NewJwtResolver(testSecret).CreateToken("user-1", nil, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	return tokens.AccessToken
}

func wrongSignatureToken(t *testing.T) string {
	t.Helper()
	tokens, err := jwtresolver.NewJwtResolver("not-the-server-secret").CreateToken("user-1", nil, time.Now())
	require.NoError(t, err)
	return tokens.AccessToken
}

func requireUnauthorizedJSON(t *testing.T, resp *http.Response, body []byte) {
	t.Helper()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode, "body: %s", body)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	var errResp api.ErrorResponse
	require.NoError(t, json.Unmarshal(body, &errResp))
	assert.Equal(t, "Unauthorized", errResp.Message, "원인 미노출 일반 문구")
}

func TestAuthContract_InvalidToken(t *testing.T) {
	// AC: 무효 토큰은 원인 불문 401 + ErrorResponse JSON (기존: 만료 401·그 외 500, 전부 빈 본문)
	tokens := []struct {
		name  string
		token string
	}{
		{name: "만료", token: expiredToken(t)},
		{name: "형식 불량(malformed)", token: "undefined"},
		{name: "서명 불일치", token: wrongSignatureToken(t)},
	}

	// 전역 미들웨어의 판정이므로 공개 엔드포인트에서도 동일하다
	endpoints := []struct {
		name string
		path string
	}{
		{name: "보호(scrap-job)", path: "/api/my/scrap-job"},
		{name: "공개(job_postings)", path: "/api/job_postings"},
	}

	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			for _, tc := range tokens {
				t.Run(tc.name, func(t *testing.T) {
					f := newFakes()
					server, _ := newStrictTestServer(t, f)

					resp, body := doRequest(t, http.MethodGet, server.URL+ep.path, tc.token, nil)
					requireUnauthorizedJSON(t, resp, body)
				})
			}
		})
	}
}

func TestAuthContract_LoginRequiredBySpecSecurity(t *testing.T) {
	// AC: 로그인 필수 401은 명세 security 선언 기반 미들웨어(AuthenticationFunc)에서 발신된다.
	// scrapJob·matchJob 전 연산이 대상이다 — 핸들러의 로그인 분기는 제거됐다.
	endpoints := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/api/my/scrap-job", nil},
		{http.MethodPost, "/api/my/scrap-job", map[string]any{"site": "s", "postingId": "p"}},
		{http.MethodDelete, "/api/my/scrap-job", map[string]any{"site": "s", "postingId": "p"}},
		{http.MethodGet, "/api/my/scrap-job/untagged", nil},
		{http.MethodGet, "/api/my/scrap-job/tags", nil},
		{http.MethodPost, "/api/my/scrap-job/tags", map[string]any{"site": "s", "postingId": "p", "tag": "t"}},
		{http.MethodDelete, "/api/my/scrap-job/tags", map[string]any{"site": "s", "postingId": "p", "tag": "t"}},
		{http.MethodGet, "/api/my/match-job", nil},
	}

	f := newFakes()
	server, _ := newStrictTestServer(t, f)

	for _, ep := range endpoints {
		resp, body := doRequest(t, ep.method, server.URL+ep.path, "", ep.body)
		requireUnauthorizedJSON(t, resp, body)
	}
}

func TestAuthContract_PublicEndpointWithoutLogin(t *testing.T) {
	// AC: 공개 엔드포인트(security `{}` 대안 선언)는 여전히 비로그인 접근 가능하다
	f := newFakes()
	f.posting.categories = &postingGrpc.CategoriesResponse{}
	server, _ := newStrictTestServer(t, f)

	resp, body := doRequest(t, http.MethodGet, server.URL+"/api/categories", "", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", body)
}
