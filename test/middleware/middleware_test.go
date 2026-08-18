package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/jwtresolver"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/middleware"
	"github.com/jae2274/careerhub-api-composer/test/tokentest"
	"github.com/jae2274/goutils/mw/httpmw"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SetClaimsMW 단위 테스트 — claims 주입과 무효 토큰 401(원인 불문 · ErrorResponse JSON).
// 로그인 필수·권한 검사는 미들웨어 함수가 아니라 명세 기반 배선의 관할이다(계약 테스트가 고정).
func TestSetClaimsMW(t *testing.T) {
	zitadel := tokentest.NewFakeZitadel(t)
	server := initServer(t, zitadel)
	defer server.Close()

	getBody := func(res *http.Response) string {
		body, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		return string(body)
	}

	t.Run("토큰 없음 — 통과, claims 미주입", func(t *testing.T) {
		res, err := http.Get(server.URL + "/hello")
		require.NoError(t, err)
		defer res.Body.Close()
		require.Equal(t, "Hello, World!", getBody(res))
	})

	t.Run("유효 토큰 — claims 주입", func(t *testing.T) {
		req, err := http.NewRequest("GET", server.URL+"/hello", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+zitadel.IssueAccessToken("Jyo Liar", nil))

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()
		require.Equal(t, "Hello, Jyo Liar!", getBody(res))
	})

	t.Run("무효 토큰 — 원인 불문 401 + ErrorResponse JSON", func(t *testing.T) {
		cases := []struct {
			name  string
			token string
		}{
			{name: "만료", token: "Bearer " + createExpiredToken(t, zitadel, "Jyo Liar")},
			{name: "형식 불량(malformed)", token: "Bearer undefined"},
			{name: "서명 불일치", token: "Bearer " + createWrongKeyToken(t, zitadel, "Jyo Liar")},
			{name: "Bearer prefix 없는 raw 토큰", token: zitadel.IssueAccessToken("Jyo Liar", nil)},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				req, err := http.NewRequest("GET", server.URL+"/hello", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", tc.token)

				res, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				defer res.Body.Close()

				require.Equal(t, http.StatusUnauthorized, res.StatusCode)
				assert.Equal(t, "application/json", res.Header.Get("Content-Type"))
				var errResp api.ErrorResponse
				require.NoError(t, json.NewDecoder(res.Body).Decode(&errResp))
				assert.Equal(t, "Unauthorized", errResp.Message, "원인 미노출 일반 문구")
			})
		}
	})
}

func initServer(t *testing.T, zitadel *tokentest.FakeZitadel) *httptest.Server {
	jr := zitadel.NewResolver(t)

	router := mux.NewRouter()
	router.Use(httpmw.SetTraceIdMW(), middleware.SetClaimsMW(jr))
	router.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.GetClaims(r.Context())
		if ok {
			w.Write([]byte("Hello, " + claims.UserId + "!"))
			return
		}
		w.Write([]byte("Hello, World!"))
	})

	return httptest.NewServer(router)
}

func createExpiredToken(t *testing.T, zitadel *tokentest.FakeZitadel, userId string) string {
	return zitadel.IssueToken(&jwtresolver.CustomClaims{
		RegisteredClaims: tokentest.RegisteredClaims(userId, time.Now().Add(-2*time.Hour), time.Hour),
	})
}

// createWrongKeyToken은 JWKS에 없는 다른 RSA 키로 서명한다 — 서명 불일치.
func createWrongKeyToken(t *testing.T, zitadel *tokentest.FakeZitadel, userId string) string {
	other := tokentest.NewFakeZitadel(t)
	other.Kid = zitadel.Kid // kid는 같지만 키가 다르다 — 재조회로도 통과 불가
	return other.IssueToken(&jwtresolver.CustomClaims{
		RegisteredClaims: tokentest.RegisteredClaims(userId, time.Now(), time.Hour),
	})
}
