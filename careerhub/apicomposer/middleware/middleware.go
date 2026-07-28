package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/jwtresolver"
	"github.com/jae2274/goutils/llog"
)

type claimsKey string

const claimsKeyStr claimsKey = "claims"

// MessageUnauthorized — 미들웨어 발신 401의 원인 미노출 일반 문구.
// 만료·형식 불량·서명 불일치의 구분은 로그로만 남긴다(RFC 6750 — 무효 토큰은 원인 불문 401).
const MessageUnauthorized = "Unauthorized"

func SetClaimsMW(jr *jwtresolver.JwtResolver) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.Header.Get("Authorization")
			if tokenString != "" {
				claims, isValid, err := jr.ParseToken(tokenString)
				if err != nil || !isValid {
					if err != nil {
						llog.LogErr(r.Context(), err)
					}
					WriteError(w, http.StatusUnauthorized, MessageUnauthorized)
					return
				}

				ctx := context.WithValue(r.Context(), claimsKeyStr, claims)
				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}

func GetClaims(ctx context.Context) (*jwtresolver.CustomClaims, bool) {
	claims, ok := ctx.Value(claimsKeyStr).(*jwtresolver.CustomClaims)
	return claims, ok
}

// WriteError는 미들웨어가 끊는 에러 응답을 명세의 ErrorResponse(JSON)로 쓴다.
// spec 검증 미들웨어보다 앞에서 끊는 경로도 있으므로 Content-Type을 직접 설정한다.
func WriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(api.ErrorResponse{Message: message})
}
