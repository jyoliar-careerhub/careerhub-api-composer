package jwtresolver

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Authority는 `careerhub:authorities` 클레임의 원소다.
// 스키마의 원천은 auth-service의 spec/issuance-hook/openapi.yaml(AuthorityClaim)이다.
type Authority struct {
	Code string `json:"code"`
	// ExpiresAt이 nil이면 무기한이다. 만료된 권한도 발급 시점에는 실리지 않지만,
	// 클레임은 발급 시점 스냅샷(ADR-0031)이라 토큰 수명 안에서 만료될 수 있다.
	ExpiresAt *time.Time `json:"expiresAt"`
}

// CustomClaims는 Zitadel 발급 액세스 토큰의 클레임이다.
// careerhub: prefix는 발급 훅이 싣는 네임스페이스이고, 사용자 식별은 표준 `sub`다.
type CustomClaims struct {
	UserId              string      `json:"-"` // 파싱 시 RegisteredClaims.Subject에서 복사된다
	Authorities         []Authority `json:"careerhub:authorities"`
	AgreementsFulfilled bool        `json:"careerhub:agreements_fulfilled"`
	jwt.RegisteredClaims
}

// HasAuthority는 판정 시점에 유효한(만료되지 않은) 권한만 인정한다.
// 만료 재확인을 여기 두어, 권한 검사 호출부는 만료를 몰라도 된다.
func (c *CustomClaims) HasAuthority(authority string) bool {
	now := time.Now()
	for _, a := range c.Authorities {
		if a.Code == authority && (a.ExpiresAt == nil || a.ExpiresAt.After(now)) {
			return true
		}
	}
	return false
}
