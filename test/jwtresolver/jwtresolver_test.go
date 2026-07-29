package jwtresolver

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/jwtresolver"
	"github.com/stretchr/testify/require"
)

func TestJwtresolver(t *testing.T) {
	secretKey := "testKey"
	userId := "Jyo Liar"
	authorities := []string{"admin", "user"}

	now := time.Now()
	duration := 30 * time.Minute

	t.Run("return valid claims", func(t *testing.T) {
		//Given
		accessToken := createAccessToken(t, secretKey, userId, authorities, now, duration)

		jr := jwtresolver.NewJwtResolver(secretKey)

		//When
		claims, ok, err := jr.ParseToken(accessToken)

		//Then
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, userId, claims.UserId)
		require.Equal(t, authorities, claims.Authorities)
		require.Equal(t, now.Add(duration).Unix(), claims.ExpiresAt.Time.Unix())
	})

	t.Run("return valid claims even empty authorities", func(t *testing.T) {
		//Given
		accessToken := createAccessToken(t, secretKey, userId, []string{}, now, duration)

		jr := jwtresolver.NewJwtResolver(secretKey)

		//When
		claims, ok, err := jr.ParseToken(accessToken)

		//Then
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, userId, claims.UserId)
		require.Empty(t, claims.Authorities)
		require.Equal(t, now.Add(duration).Unix(), claims.ExpiresAt.Time.Unix())
	})

	t.Run("return invalid claims if after expiresAt", func(t *testing.T) {
		//Given
		accessToken := createAccessToken(t, secretKey, userId, authorities, now.Add(-duration-time.Minute), duration)

		jr := jwtresolver.NewJwtResolver(secretKey)

		//When
		_, ok, err := jr.ParseToken(accessToken)

		//Then
		require.NoError(t, err)
		require.False(t, ok)
	})

	//아래 두 케이스는 각 서비스간 CustomClaims 및 토큰 규약이 제대로 지켜지지 않았을 때 발생하므로 에러 발생으로 처리
	t.Run("return error when secret key is different", func(t *testing.T) {
		//Given
		now := time.Now()
		accessToken := createAccessToken(t, "differentSecretKey", userId, authorities, now, duration)

		jr := jwtresolver.NewJwtResolver(secretKey)

		//When
		_, ok, err := jr.ParseToken(accessToken)

		//Then
		require.Error(t, err)
		require.False(t, ok)
	})

	t.Run("return error if empty userId", func(t *testing.T) {
		//Given
		accessToken := createAccessToken(t, secretKey, "", authorities, now, duration)

		jr := jwtresolver.NewJwtResolver(secretKey)

		//When
		_, ok, err := jr.ParseToken(accessToken)

		//Then
		require.Error(t, err)
		require.False(t, ok)
	})
}

func createAccessToken(t *testing.T, secretKey, userId string, authorities []string, now time.Time, duration time.Duration) string {

	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256,
		&jwtresolver.CustomClaims{
			UserId:      userId,
			Authorities: authorities,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    "careerhub.jyo-liar.com",              //TODO: 임의 설정
				Audience:  []string{"careerhub.jyo-liar.com"},    //TODO: 임의 설정
				ExpiresAt: jwt.NewNumericDate(now.Add(duration)), //TODO: 임의 설정
				IssuedAt:  jwt.NewNumericDate(now),
				NotBefore: jwt.NewNumericDate(now),
			},
		},
	).SignedString([]byte(secretKey))

	require.NoError(t, err)

	return accessToken
}
