package jwtresolver

import (
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/jwtresolver"
	"github.com/jae2274/careerhub-api-composer/test/tokentest"
	"github.com/stretchr/testify/require"
)

func TestParseToken(t *testing.T) {
	userId := "385923124626982939" // Zitadel sub 모양
	authorities := []string{"AUTHORITY_ADMIN", "AUTHORITY_MATCH_JOB"}

	t.Run("Zitadel 발급 모양의 토큰 수용 — sub가 UserId가 된다", func(t *testing.T) {
		zitadel := tokentest.NewFakeZitadel(t)
		jr := zitadel.NewResolver(t)

		claims, ok, err := jr.ParseToken("Bearer " + zitadel.IssueAccessToken(userId, authorities))

		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, userId, claims.UserId)
		require.True(t, claims.HasAuthority("AUTHORITY_ADMIN"))
		require.True(t, claims.HasAuthority("AUTHORITY_MATCH_JOB"))
		require.True(t, claims.AgreementsFulfilled)
	})

	t.Run("레거시 대칭키(HS256) 토큰 거부", func(t *testing.T) {
		zitadel := tokentest.NewFakeZitadel(t)
		jr := zitadel.NewResolver(t)

		// 레거시 모양 그대로: HS256 + json 태그 없는 Go 필드명 키
		legacyToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"UserId":      userId,
			"Authorities": authorities,
			"exp":         time.Now().Add(10 * time.Minute).Unix(),
		}).SignedString([]byte("legacy-secret-key"))
		require.NoError(t, err)

		_, ok, err := jr.ParseToken("Bearer " + legacyToken)

		require.Error(t, err)
		require.False(t, ok)
	})

	t.Run("alg 위조 토큰 거부 — RS256 공개키를 HS256 키로 재사용한 서명", func(t *testing.T) {
		zitadel := tokentest.NewFakeZitadel(t)
		jr := zitadel.NewResolver(t)

		// alg confusion: 검증자의 공개키(PEM)를 HMAC 비밀로 삼아 HS256 서명한다.
		// WithValidMethods로 RS256이 고정돼 있으므로 키 일치 여부 이전에 거부돼야 한다.
		pubDer, err := x509.MarshalPKIXPublicKey(zitadel.PrivateKey.Public())
		require.NoError(t, err)
		pubPem := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDer})

		forged := jwt.NewWithClaims(jwt.SigningMethodHS256,
			&jwtresolver.CustomClaims{RegisteredClaims: tokentest.RegisteredClaims(userId, time.Now(), 10*time.Minute)})
		forged.Header["kid"] = zitadel.Kid
		forgedToken, err := forged.SignedString(pubPem)
		require.NoError(t, err)

		_, ok, err := jr.ParseToken("Bearer " + forgedToken)

		require.Error(t, err)
		require.False(t, ok)
	})

	t.Run("발급자 정지 중 캐시된 키로 검증 지속", func(t *testing.T) {
		zitadel := tokentest.NewFakeZitadel(t)
		jr := zitadel.NewResolver(t) // 생성 시점 조회로 키가 캐시된다

		zitadel.SetHealthy(false) // 발급자 정지

		claims, ok, err := jr.ParseToken("Bearer " + zitadel.IssueAccessToken(userId, nil))

		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, userId, claims.UserId)
	})

	t.Run("콜드 스타트 첫 조회 실패 — 생성은 성공, 검증은 실패", func(t *testing.T) {
		zitadel := tokentest.NewFakeZitadel(t)
		zitadel.SetHealthy(false)

		jr := zitadel.NewResolver(t) // 첫 조회가 실패해도 생성은 막히지 않는다

		_, ok, err := jr.ParseToken("Bearer " + zitadel.IssueAccessToken(userId, nil))
		require.Error(t, err)
		require.False(t, ok)
	})

	t.Run("콜드 스타트 첫 조회 실패 — 발급자 복구 후 첫 검증이 키를 획득한다", func(t *testing.T) {
		zitadel := tokentest.NewFakeZitadel(t)
		zitadel.SetHealthy(false)

		jr := zitadel.NewResolver(t)

		zitadel.SetHealthy(true) // 발급자 복구

		claims, ok, err := jr.ParseToken("Bearer " + zitadel.IssueAccessToken(userId, nil))
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, userId, claims.UserId)
	})

	t.Run("키 회전(kid) 추종 — 새 kid 토큰이 재조회로 수용된다", func(t *testing.T) {
		zitadel := tokentest.NewFakeZitadel(t)
		jr := zitadel.NewResolver(t) // kid-1이 캐시된다

		zitadel.RotateSigningKey("test-kid-2")

		claims, ok, err := jr.ParseToken("Bearer " + zitadel.IssueAccessToken(userId, nil))
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, userId, claims.UserId)
	})

	t.Run("iss 불일치 거부", func(t *testing.T) {
		zitadel := tokentest.NewFakeZitadel(t)
		jr := zitadel.NewResolver(t)

		registered := tokentest.RegisteredClaims(userId, time.Now(), 10*time.Minute)
		registered.Issuer = "https://not-the-issuer.test"
		token := zitadel.IssueToken(&jwtresolver.CustomClaims{RegisteredClaims: registered})

		_, ok, err := jr.ParseToken("Bearer " + token)
		require.Error(t, err)
		require.False(t, ok)
	})

	t.Run("aud 불일치 거부", func(t *testing.T) {
		zitadel := tokentest.NewFakeZitadel(t)
		jr := zitadel.NewResolver(t)

		registered := tokentest.RegisteredClaims(userId, time.Now(), 10*time.Minute)
		registered.Audience = jwt.ClaimStrings{"some-other-client"}
		token := zitadel.IssueToken(&jwtresolver.CustomClaims{RegisteredClaims: registered})

		_, ok, err := jr.ParseToken("Bearer " + token)
		require.Error(t, err)
		require.False(t, ok)
	})

	t.Run("만료 토큰 — 에러 없이 무효", func(t *testing.T) {
		zitadel := tokentest.NewFakeZitadel(t)
		jr := zitadel.NewResolver(t)

		token := zitadel.IssueToken(&jwtresolver.CustomClaims{
			RegisteredClaims: tokentest.RegisteredClaims(userId, time.Now().Add(-time.Hour), 10*time.Minute)})

		_, ok, err := jr.ParseToken("Bearer " + token)
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("Bearer prefix 없는 raw 토큰 거부", func(t *testing.T) {
		zitadel := tokentest.NewFakeZitadel(t)
		jr := zitadel.NewResolver(t)

		_, ok, err := jr.ParseToken(zitadel.IssueAccessToken(userId, nil))
		require.Error(t, err)
		require.False(t, ok)
	})

	t.Run("sub 없는 토큰 거부", func(t *testing.T) {
		zitadel := tokentest.NewFakeZitadel(t)
		jr := zitadel.NewResolver(t)

		token := zitadel.IssueToken(&jwtresolver.CustomClaims{
			RegisteredClaims: tokentest.RegisteredClaims("", time.Now(), 10*time.Minute)})

		_, ok, err := jr.ParseToken("Bearer " + token)
		require.Error(t, err)
		require.False(t, ok)
	})
}

func TestHasAuthority(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)

	claims := &jwtresolver.CustomClaims{Authorities: []jwtresolver.Authority{
		{Code: "AUTHORITY_UNLIMITED", ExpiresAt: nil},
		{Code: "AUTHORITY_VALID", ExpiresAt: &future},
		{Code: "AUTHORITY_EXPIRED", ExpiresAt: &past},
	}}

	require.True(t, claims.HasAuthority("AUTHORITY_UNLIMITED"), "expiresAt null은 무기한")
	require.True(t, claims.HasAuthority("AUTHORITY_VALID"))
	require.False(t, claims.HasAuthority("AUTHORITY_EXPIRED"), "토큰 수명 안에서 만료된 권한은 판정 시점에 걸러진다")
	require.False(t, claims.HasAuthority("AUTHORITY_ABSENT"))
}

// 클레임 json 매핑 — careerhub: 네임스페이스 키와 {code, expiresAt} 원소 모양이
// 발급 훅 스키마(auth-service spec/issuance-hook AuthorityClaim)와 일치해야 한다.
func TestClaimsJsonMapping(t *testing.T) {
	zitadel := tokentest.NewFakeZitadel(t)
	jr := zitadel.NewResolver(t)

	expiresAt := time.Date(2026, 12, 31, 15, 0, 0, 0, time.UTC)
	token := zitadel.IssueToken(jwt.MapClaims{
		"iss": tokentest.Issuer,
		"aud": tokentest.Audience,
		"sub": "385923124626982939",
		"exp": time.Now().Add(10 * time.Minute).Unix(),
		"careerhub:authorities": []map[string]any{
			{"code": "AUTHORITY_MATCH_JOB", "expiresAt": "2026-12-31T15:00:00Z"},
			{"code": "AUTHORITY_ADMIN", "expiresAt": nil},
		},
		"careerhub:agreements_fulfilled": true,
	})

	claims, ok, err := jr.ParseToken("Bearer " + token)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, []jwtresolver.Authority{
		{Code: "AUTHORITY_MATCH_JOB", ExpiresAt: &expiresAt},
		{Code: "AUTHORITY_ADMIN", ExpiresAt: nil},
	}, claims.Authorities)
	require.True(t, claims.AgreementsFulfilled)
}
