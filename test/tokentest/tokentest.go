// Package tokentest는 Zitadel 발급 경로를 흉내내는 테스트 하네스다:
// 임의 RSA 키쌍을 만들고, private key로 토큰을 서명하며, public key를
// 가짜 JWKS 엔드포인트로 서빙한다. 서명 능력은 테스트에만 존재한다 —
// 게이트웨이 코드에는 서명 경로가 없다.
package tokentest

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/jwtresolver"
	"github.com/stretchr/testify/require"
)

const (
	Issuer   = "https://id.careerhub.test"
	Audience = "careerhub-web-client-id"
)

// Jwks는 가짜 JWKS 엔드포인트와 그 서명 키를 묶는다.
// 셋업 실패는 panic으로 낸다 — 테스트 하네스 한정의 선택이다.
type Jwks struct {
	PrivateKey *rsa.PrivateKey
	Kid        string
	Server     *httptest.Server
	healthy    atomic.Bool
	jwksJSON   atomic.Value // []byte
}

// Start는 하네스를 만든다. 호출자가 Close를 책임진다 — 테스트 단위 수명이면 NewJwks를 쓴다.
func Start() *Jwks {
	j := &Jwks{}
	j.healthy.Store(true)
	j.RotateKey("test-kid-1")
	j.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !j.healthy.Load() {
			http.Error(w, "issuer down", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(j.jwksJSON.Load().([]byte))
	}))
	return j
}

func NewJwks(t *testing.T) *Jwks {
	t.Helper()
	j := Start()
	t.Cleanup(j.Close)
	return j
}

func (j *Jwks) Close() {
	j.Server.Close()
}

// RotateKey는 새 RSA 키쌍을 생성해 서빙 중인 JWKS를 통째로 교체한다 — 키 회전(kid) 추종 재현용.
func (j *Jwks) RotateKey(kid string) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	j.PrivateKey = key
	j.Kid = kid

	jwk, err := jwkset.NewJWKFromKey(key.Public(), jwkset.JWKOptions{
		Metadata: jwkset.JWKMetadataOptions{ALG: jwkset.AlgRS256, KID: kid, USE: jwkset.UseSig},
	})
	if err != nil {
		panic(err)
	}
	store := jwkset.NewMemoryStorage()
	if err := store.KeyWrite(context.Background(), jwk); err != nil {
		panic(err)
	}
	raw, err := store.JSONPublic(context.Background())
	if err != nil {
		panic(err)
	}
	j.jwksJSON.Store([]byte(raw))
}

// SetHealthy(false)면 JWKS 엔드포인트가 503을 낸다 — 발급자 정지·콜드 스타트 첫 조회 실패 재현용.
func (j *Jwks) SetHealthy(healthy bool) {
	j.healthy.Store(healthy)
}

func (j *Jwks) NewResolver(t *testing.T) *jwtresolver.JwtResolver {
	t.Helper()
	jr, err := jwtresolver.NewJwtResolver(t.Context(), jwtresolver.Config{
		Issuer:   Issuer,
		Audience: Audience,
		JwksUrl:  j.Server.URL,
	})
	require.NoError(t, err)
	return jr
}

// SignToken은 임의 클레임을 현재 키로 RS256 서명한다.
func (j *Jwks) SignToken(claims jwt.Claims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = j.Kid
	signed, err := token.SignedString(j.PrivateKey)
	if err != nil {
		panic(err)
	}
	return signed
}

// AccessToken은 Zitadel 발급 모양(iss·aud·sub·careerhub 클레임)의 액세스 토큰을 만든다.
// 권한 코드들은 무기한(expiresAt=null) 권한으로 실린다.
func (j *Jwks) AccessToken(userID string, authorityCodes []string) string {
	authorities := make([]jwtresolver.Authority, 0, len(authorityCodes))
	for _, code := range authorityCodes {
		authorities = append(authorities, jwtresolver.Authority{Code: code})
	}
	return j.SignToken(&jwtresolver.CustomClaims{
		Authorities:         authorities,
		AgreementsFulfilled: true,
		RegisteredClaims:    RegisteredClaims(userID, time.Now(), 30*time.Minute),
	})
}

// RegisteredClaims는 검증을 통과하는 표준 클레임 묶음을 만든다.
func RegisteredClaims(userID string, now time.Time, duration time.Duration) jwt.RegisteredClaims {
	return jwt.RegisteredClaims{
		Issuer:    Issuer,
		Audience:  jwt.ClaimStrings{Audience},
		Subject:   userID,
		ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
		IssuedAt:  jwt.NewNumericDate(now),
	}
}
