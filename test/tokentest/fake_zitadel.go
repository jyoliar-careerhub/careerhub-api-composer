// Package tokentest는 테스트에서 Zitadel의 자리를 대신하는 FakeZitadel을 제공한다.
//
// 실제 Zitadel이 하는 두 가지 역할을 그대로 흉내낸다:
//
//  1. 토큰 발급 — 자기만 아는 RSA private key로 액세스 토큰(JWT)을 RS256 서명해 내려준다.
//  2. JWKS 엔드포인트 — 그 키쌍의 public key를 HTTP(JWKS JSON)로 공개한다.
//
// 검증자(api-composer의 JwtResolver)는 토큰 헤더의 kid로 ②에서 받은 공개키를 찾아
// ①의 서명을 검증한다 — 프로드에서 일어나는 일과 같은 배선이다.
// 서명 능력(private key)은 이 하네스 안에만 존재한다 — 게이트웨이 코드에는 서명 경로가 없다.
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

// 발급 구성 — 실제 배포의 OIDC_ISSUER·OIDC_AUDIENCE에 대응하는 테스트 고정값.
const (
	Issuer   = "https://id.careerhub.test"
	Audience = "careerhub-web-client-id"
)

// FakeZitadel은 토큰 발급자와 JWKS 엔드포인트를 묶은 가짜 Zitadel이다.
// 셋업 실패는 panic으로 낸다 — 테스트 하네스 한정의 선택이다.
type FakeZitadel struct {
	PrivateKey *rsa.PrivateKey // 현재 서명 키 — public 쪽만 JWKS로 공개된다
	Kid        string          // 현재 서명 키의 key id — 발급 토큰의 헤더에 실린다
	server     *httptest.Server
	healthy    atomic.Bool
	jwksJSON   atomic.Value // []byte — 서빙 중인 JWKS 응답 본문
}

// StartFakeZitadel은 발급 키를 만들고 JWKS 엔드포인트를 연다.
// 호출자가 Close를 책임진다 — 테스트 단위 수명이면 NewFakeZitadel을 쓴다.
func StartFakeZitadel() *FakeZitadel {
	z := &FakeZitadel{}
	z.healthy.Store(true)
	z.RotateSigningKey("test-kid-1")
	z.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !z.healthy.Load() {
			http.Error(w, "issuer down", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(z.jwksJSON.Load().([]byte))
	}))
	return z
}

func NewFakeZitadel(t *testing.T) *FakeZitadel {
	t.Helper()
	z := StartFakeZitadel()
	t.Cleanup(z.Close)
	return z
}

func (z *FakeZitadel) Close() {
	z.server.Close()
}

// JwksURL은 JWKS 엔드포인트 주소다 — 실제 Zitadel의 {issuer}/oauth/v2/keys 자리.
func (z *FakeZitadel) JwksURL() string {
	return z.server.URL
}

// RotateSigningKey는 새 RSA 키쌍을 만들어 서명 키와 JWKS 공개분을 통째로 교체한다.
// 이후 발급되는 토큰은 새 kid를 달고 나온다 — 키 회전 추종 시나리오 재현용.
func (z *FakeZitadel) RotateSigningKey(kid string) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	z.PrivateKey = key
	z.Kid = kid

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
	z.jwksJSON.Store([]byte(raw))
}

// SetHealthy(false)면 JWKS 엔드포인트가 503을 낸다 — 발급자 정지·콜드 스타트 첫 조회 실패 재현용.
// 토큰 발급(IssueToken 계열)은 영향받지 않는다 — 정지 전에 발급된 토큰이 남아 있는 상황의 재현이 목적이다.
func (z *FakeZitadel) SetHealthy(healthy bool) {
	z.healthy.Store(healthy)
}

// NewResolver는 이 발급자를 신뢰하도록 구성된 검증기를 만든다 — api-composer가
// 배포 env(OIDC_ISSUER 등)로 실제 Zitadel을 가리키는 것에 대응한다.
func (z *FakeZitadel) NewResolver(t *testing.T) *jwtresolver.JwtResolver {
	t.Helper()
	jr, err := jwtresolver.NewJwtResolver(t.Context(), jwtresolver.Config{
		Issuer:   Issuer,
		Audience: Audience,
		JwksUrl:  z.JwksURL(),
	})
	require.NoError(t, err)
	return jr
}

// IssueToken은 임의 클레임을 현재 키로 RS256 서명해 발급한다 — iss/aud 불일치·만료 등
// 정상 발급 밖의 모양이 필요한 테스트용 저수준 입구.
func (z *FakeZitadel) IssueToken(claims jwt.Claims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = z.Kid
	signed, err := token.SignedString(z.PrivateKey)
	if err != nil {
		panic(err)
	}
	return signed
}

// IssueAccessToken은 정상 로그인 결과에 대응하는 액세스 토큰을 발급한다:
// iss·aud·sub 표준 클레임 + 발급 훅이 싣는 careerhub 클레임.
// 권한 코드들은 무기한(expiresAt=null) 권한으로 실린다.
func (z *FakeZitadel) IssueAccessToken(userID string, authorityCodes []string) string {
	authorities := make([]jwtresolver.Authority, 0, len(authorityCodes))
	for _, code := range authorityCodes {
		authorities = append(authorities, jwtresolver.Authority{Code: code})
	}
	return z.IssueToken(&jwtresolver.CustomClaims{
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
