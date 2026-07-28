package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/jwtresolver"
	reviewGrpc "github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/review/restapi_grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewStrict_Success(t *testing.T) {
	unixMilli := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC).UnixMilli()
	f := newFakes()
	f.review.reviews = []*reviewGrpc.Review{{
		Score:            4,
		Summary:          "좋은 회사",
		EmploymentStatus: true,
		ReviewUserId:     "user-1",
		JobType:          "백엔드",
		UnixMilli:        unixMilli,
	}}
	server, spanRecorder := newStrictTestServer(t, f)

	resp, body := doRequest(t, http.MethodGet, server.URL+"/api/company-review/reviews?companyName=acme&page=2&size=5", "", nil)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")

	var reviews []api.Review
	require.NoError(t, json.Unmarshal(body, &reviews))
	require.Len(t, reviews, 1)
	assert.Equal(t, int32(4), reviews[0].Score)
	assert.Equal(t, "좋은 회사", reviews[0].Summary)
	assert.True(t, reviews[0].EmploymentStatus)
	assert.Equal(t, unixMilli, reviews[0].Date.UnixMilli())

	// 성공 계약 불변: 기존 핸들러와 같은 페이지네이션 변환(offset = page*size)
	require.NotNil(t, f.review.gotReviews)
	assert.Equal(t, "acme", f.review.gotReviews.CompanyName)
	assert.Equal(t, int64(10), f.review.gotReviews.Offset)
	assert.Equal(t, int64(5), f.review.gotReviews.Limit)

	// 관측 유지: otelmux 스팬 + trace_id의 ctx 관통
	require.NotEmpty(t, spanRecorder.Ended())
	assert.Len(t, f.review.capture.traceID, 32)
}

func TestReviewStrict_DefaultPaging(t *testing.T) {
	f := newFakes()
	server, _ := newStrictTestServer(t, f)

	resp, _ := doRequest(t, http.MethodGet, server.URL+"/api/company-review/reviews?companyName=acme", "", nil)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotNil(t, f.review.gotReviews)
	assert.Equal(t, int64(0), f.review.gotReviews.Offset)
	assert.Equal(t, int64(10), f.review.gotReviews.Limit)
}

func TestReviewStrict_ErrorContract(t *testing.T) {
	cases := []struct {
		name        string
		query       string
		fakeErr     error
		wantStatus  int
		wantMessage string // 빈 문자열이면 message 존재만 확인 (명세 검증기의 문구는 고정하지 않는다)
	}{
		{name: "음수 page(명세 minimum)", query: "?companyName=acme&page=-1", wantStatus: 400},
		{name: "음수 size(명세 minimum)", query: "?companyName=acme&size=-1", wantStatus: 400},
		{name: "빈 companyName(명세 minLength)", query: "?companyName=", wantStatus: 400},
		{name: "companyName 누락(명세 required)", query: "?page=0", wantStatus: 400},
		{name: "비정수 page(명세 type)", query: "?companyName=acme&page=abc", wantStatus: 400},
		{name: "업스트림 오류", query: "?companyName=acme", fakeErr: context.DeadlineExceeded, wantStatus: 500, wantMessage: "Internal Server Error"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakes()
			f.review.reviewsErr = tc.fakeErr
			server, _ := newStrictTestServer(t, f)

			resp, body := doRequest(t, http.MethodGet, server.URL+"/api/company-review/reviews"+tc.query, "", nil)

			require.Equal(t, tc.wantStatus, resp.StatusCode)
			// 에러 계약: text/plain이 아니라 구조화 JSON(ErrorResponse)
			assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
			var errResp api.ErrorResponse
			require.NoError(t, json.Unmarshal(body, &errResp), "body: %s", body)
			require.NotEmpty(t, errResp.Message)
			if tc.wantMessage != "" {
				assert.Equal(t, tc.wantMessage, errResp.Message)
			}
		})
	}
}

func TestReviewStrict_Auth(t *testing.T) {
	t.Run("유효 토큰이면 claims가 서비스 계층 ctx까지 관통한다", func(t *testing.T) {
		f := newFakes()
		server, _ := newStrictTestServer(t, f)

		resp, _ := doRequest(t, http.MethodGet, server.URL+"/api/company-review/reviews?companyName=acme", newToken(t, "test-user", []string{"AUTHORITY_READ_REVIEW"}), nil)

		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.NotNil(t, f.review.capture.claims)
		assert.Equal(t, "test-user", f.review.capture.claims.UserId)
	})

	t.Run("만료 토큰이면 미들웨어가 401로 끊는다 (ErrorResponse JSON)", func(t *testing.T) {
		f := newFakes()
		server, _ := newStrictTestServer(t, f)

		tokens, err := jwtresolver.NewJwtResolver(testSecret).CreateToken("test-user", nil, time.Now().Add(-24*time.Hour))
		require.NoError(t, err)

		resp, body := doRequest(t, http.MethodGet, server.URL+"/api/company-review/reviews?companyName=acme", tokens.AccessToken, nil)

		requireUnauthorizedJSON(t, resp, body)
		assert.Nil(t, f.review.gotReviews)
	})
}
