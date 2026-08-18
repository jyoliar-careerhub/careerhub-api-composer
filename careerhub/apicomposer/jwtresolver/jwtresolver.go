package jwtresolver

import (
	"context"
	"errors"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jae2274/goutils/terr"
)

// Config는 Zitadel JWKS 로컬 검증(ADR-0029)의 좌표다.
type Config struct {
	Issuer   string // `iss` 기대값 (예: https://id.careerhub.jyo-liar.com)
	Audience string // `aud`에 포함돼야 하는 값 (careerhub-web client_id 등)
	JwksUrl  string // JWKS 엔드포인트. 비면 {Issuer}/oauth/v2/keys (Zitadel 규약)
}

type JwtResolver struct {
	keyfunc keyfunc.Keyfunc
	parser  *jwt.Parser
}

// NewJwtResolver는 JWKS 캐시(제어 경로)를 배선한다. 키 획득 실패 모드는 keyfunc의
// 기본값이 다룬다: 콜드 스타트 첫 조회 실패는 생성을 막지 않고(빈 캐시로 시작),
// 미지 `kid`는 rate limit 안에서 재조회하며(키 회전 추종·콜드 스타트 복구),
// 주기 갱신 실패 시 캐시된 키가 유지된다(발급자 정지 중 검증 지속).
func NewJwtResolver(ctx context.Context, config Config) (*JwtResolver, error) {
	jwksUrl := config.JwksUrl
	if jwksUrl == "" {
		jwksUrl = strings.TrimSuffix(config.Issuer, "/") + "/oauth/v2/keys"
	}

	kf, err := keyfunc.NewDefaultCtx(ctx, []string{jwksUrl})
	if err != nil {
		return nil, terr.Wrap(err)
	}

	return &JwtResolver{
		keyfunc: kf,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}), // alg confusion 차단 — RS256 고정
			jwt.WithIssuer(config.Issuer),
			jwt.WithAudience(config.Audience),
			jwt.WithExpirationRequired(),
		),
	}, nil
}

// ParseToken은 `Authorization` 헤더 값을 받아 서명·`iss`·`aud`·`exp`를 로컬에서 검증한다.
// 반환: (claims, 유효 여부, 에러). 만료·미발효는 에러 없이 무효로, 그 외 무효는 에러로 구분한다
// — 호출부(SetClaimsMW)는 어느 쪽이든 401을 내고, 구분은 로그로만 남는다.
func (j *JwtResolver) ParseToken(tokenString string) (*CustomClaims, bool, error) {
	// `Bearer ` prefix 강제 (RFC 6750) — prefix 없는 raw 토큰은 형식 불량이다.
	tokenString, found := strings.CutPrefix(tokenString, "Bearer ")
	if !found {
		return &CustomClaims{}, false, terr.New("invalid token. Bearer prefix is required")
	}

	jwtToken, err := j.parser.ParseWithClaims(tokenString, &CustomClaims{}, j.keyfunc.Keyfunc)

	if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) {
		return &CustomClaims{}, false, nil
	} else if errors.Is(err, jwt.ErrTokenMalformed) {
		return &CustomClaims{}, false, terr.New("invalid token. token is malformed")
	} else if err != nil {
		return &CustomClaims{}, false, terr.Wrap(err)
	} else if jwtToken.Valid {
		claims, ok := jwtToken.Claims.(*CustomClaims)
		if !ok {
			return &CustomClaims{}, false, terr.New("invalid token. claims is not CustomClaims type")
		}
		if claims.Subject == "" {
			return &CustomClaims{}, false, terr.New("invalid token. subject(sub) is empty")
		}
		claims.UserId = claims.Subject
		return claims, true, nil
	} else {
		return &CustomClaims{}, false, nil
	}
}
