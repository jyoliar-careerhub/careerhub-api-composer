package tokentest_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/jae2274/careerhub-api-composer/test/tokentest"
	"github.com/stretchr/testify/require"
)

// FakeZitadel의 자기 실증이자 시나리오 문서다 — 프로드에서 api-composer와
// 실제 Zitadel 사이에 일어나는 일을 mock 위에서 순서대로 밟는다:
//
//	① 발급자가 액세스 토큰(JWT)을 RS256 서명해 발급한다 (사용자 로그인의 결과물)
//	② 검증자가 JWKS 엔드포인트를 HTTP로 호출해 공개키 목록을 받는다 (캐시되는 제어 경로)
//	③ 토큰 헤더의 kid로 ②의 공개키를 찾아 ①의 서명을 검증한다 (요청 경로 — 전부 로컬)
func TestFakeZitadel_시나리오_발급과_JWKS_공개(t *testing.T) {
	zitadel := tokentest.NewFakeZitadel(t)

	// ① 토큰 발급 — private key는 발급자 안에만 있다
	token := zitadel.IssueAccessToken("user-1", []string{"AUTHORITY_ADMIN"})

	// JWT 헤더에는 어떤 키로 서명했는지(kid)와 알고리즘(alg)이 실린다
	headerRaw, err := base64.RawURLEncoding.DecodeString(strings.Split(token, ".")[0])
	require.NoError(t, err)
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	require.NoError(t, json.Unmarshal(headerRaw, &header))
	require.Equal(t, "RS256", header.Alg)
	require.Equal(t, zitadel.Kid, header.Kid)

	// ② JWKS 엔드포인트를 HTTP로 직접 호출 — 검증자가 공개키를 얻는 경로 그대로
	resp, err := http.Get(zitadel.JwksURL())
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var jwks struct {
		Keys []struct {
			Kty string `json:"kty"`
			Use string `json:"use"`
			Alg string `json:"alg"`
			Kid string `json:"kid"`
			N   string `json:"n"` // RSA modulus — 공개키의 실체
			E   string `json:"e"` // RSA exponent
			D   string `json:"d"` // private exponent — 공개분에는 없어야 한다
		} `json:"keys"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&jwks))
	require.Len(t, jwks.Keys, 1)

	key := jwks.Keys[0]
	require.Equal(t, "RSA", key.Kty)
	require.Equal(t, "sig", key.Use)
	require.Equal(t, "RS256", key.Alg)
	require.Equal(t, header.Kid, key.Kid, "토큰 헤더의 kid가 JWKS의 키를 가리킨다")
	require.NotEmpty(t, key.N)
	require.NotEmpty(t, key.E)
	require.Empty(t, key.D, "JWKS는 공개키만 노출한다 — 서명 능력은 발급자 밖으로 나가지 않는다")

	// ③ 검증자는 ②의 공개키로 ①을 로컬 검증한다
	jr := zitadel.NewResolver(t)
	claims, ok, err := jr.ParseToken("Bearer " + token)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "user-1", claims.UserId)
	require.True(t, claims.HasAuthority("AUTHORITY_ADMIN"))
}

// 키 회전 후에는 JWKS가 새 kid의 공개키를 서빙하고, 발급되는 토큰도 새 kid를 단다.
func TestFakeZitadel_키_회전이_JWKS와_발급에_함께_반영된다(t *testing.T) {
	zitadel := tokentest.NewFakeZitadel(t)
	oldKid := zitadel.Kid

	zitadel.RotateSigningKey("test-kid-2")

	resp, err := http.Get(zitadel.JwksURL())
	require.NoError(t, err)
	defer resp.Body.Close()
	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
		} `json:"keys"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&jwks))
	require.Len(t, jwks.Keys, 1)
	require.Equal(t, "test-kid-2", jwks.Keys[0].Kid)
	require.NotEqual(t, oldKid, jwks.Keys[0].Kid)

	token := zitadel.IssueAccessToken("user-1", nil)
	headerRaw, err := base64.RawURLEncoding.DecodeString(strings.Split(token, ".")[0])
	require.NoError(t, err)
	var header struct {
		Kid string `json:"kid"`
	}
	require.NoError(t, json.Unmarshal(headerRaw, &header))
	require.Equal(t, "test-kid-2", header.Kid)
}

// 발급자 정지(SetHealthy false)는 JWKS 엔드포인트만 죽인다 — 503.
func TestFakeZitadel_정지_상태의_JWKS는_503(t *testing.T) {
	zitadel := tokentest.NewFakeZitadel(t)
	zitadel.SetHealthy(false)

	resp, err := http.Get(zitadel.JwksURL())
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}
