package controller

import (
	"testing"

	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/review"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 명세의 default 선언과 코드 상수의 정합성 검사 (#14 패턴 보강 코멘트).
// default 주입은 도구가 자동화하지 않아 코드 소유로 남는다 — 어긋나면 여기서 잡는다.
func TestSpecDefaults_MatchCodeConstants(t *testing.T) {
	swagger, err := api.GetSwagger()
	require.NoError(t, err)

	op := swagger.Paths.Find("/company-review/reviews").Get
	require.NotNil(t, op)

	defaults := map[string]float64{}
	for _, paramRef := range op.Parameters {
		param := paramRef.Value
		if param.Schema != nil && param.Schema.Value.Default != nil {
			defaults[param.Name] = param.Schema.Value.Default.(float64)
		}
	}

	require.Contains(t, defaults, "page")
	require.Contains(t, defaults, "size")
	assert.Equal(t, float64(review.InitPage), defaults["page"], "명세의 page default ≠ review.InitPage")
	assert.Equal(t, float64(review.DefaultSize), defaults["size"], "명세의 size default ≠ review.DefaultSize")
}
