package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/common/user_authority"
	postingGrpc "github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/posting/restapi_grpc"
	scrapGrpc "github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/userinfo/restapi_grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 보강 데이터 열화(degradation) 경로 테스트 — 서비스 계층의 설계 의도를 고정한다:
// 채용공고 조회의 보강 데이터(스크랩·리뷰 점수)는 업스트림 장애 시 로그만 남기고
// 부분 응답으로 계속한다(200). 반면 스크랩 목록 조회의 보강은 하드 실패(500)다 —
// 이 비대칭도 현행 동작이므로 그대로 고정한다.

func fullPosting() []*postingGrpc.JobPostingRes {
	return []*postingGrpc.JobPostingRes{{
		Site: "wanted", PostingId: "p1", Title: "백엔드", CompanyName: "acme", Status: "hiring",
	}}
}

func TestJobPostingsDegradation_ScrapUpstreamDown(t *testing.T) {
	// 로그인 상태에서 스크랩 업스트림이 죽어도 목록은 200 부분 응답 — scrapInfo만 null
	f := newFakes()
	f.posting.jobPostings = fullPosting()
	f.scrapJob.byIdErr = context.DeadlineExceeded
	server, _ := newStrictTestServer(t, f)

	query := encodeQuery(t, map[string]any{})
	resp, body := doRequest(t, http.MethodGet, server.URL+"/api/job_postings?page=1&size=10&encoded_query="+query, newToken(t, "user-1", nil), nil)

	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", body)
	var raw []map[string]any
	require.NoError(t, json.Unmarshal(body, &raw))
	require.Len(t, raw, 1)
	assert.Equal(t, "백엔드", raw[0]["title"]) // 본문 데이터는 온전
	v, present := raw[0]["scrapInfo"]
	assert.True(t, present)
	assert.Nil(t, v) // 보강만 열화 — null
}

func TestJobPostingsDegradation_ReviewScoresDown(t *testing.T) {
	// 리뷰 점수 업스트림이 죽어도 목록은 200 — reviewInfo만 생략
	f := newFakes()
	f.posting.jobPostings = fullPosting()
	f.review.scoresErr = context.DeadlineExceeded
	server, _ := newStrictTestServer(t, f)

	query := encodeQuery(t, map[string]any{})
	resp, body := doRequest(t, http.MethodGet, server.URL+"/api/job_postings?page=1&size=10&encoded_query="+query, "", nil)

	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", body)
	var raw []map[string]any
	require.NoError(t, json.Unmarshal(body, &raw))
	require.Len(t, raw, 1)
	assert.Equal(t, "백엔드", raw[0]["title"])
	assert.NotContains(t, raw[0], "reviewInfo")
}

func TestJobPostingDetailDegradation(t *testing.T) {
	newDetail := func() *postingGrpc.JobPostingDetailRes {
		return &postingGrpc.JobPostingDetailRes{
			Site: "wanted", PostingId: "p1", Title: "백엔드", CompanyName: "acme", Status: "hiring",
		}
	}

	t.Run("스크랩 업스트림 장애 — 200, scrapInfo null", func(t *testing.T) {
		f := newFakes()
		f.posting.detail = newDetail()
		f.scrapJob.byIdErr = context.DeadlineExceeded
		server, _ := newStrictTestServer(t, f)

		resp, body := doRequest(t, http.MethodGet, server.URL+"/api/job_postings/wanted/p1", newToken(t, "user-1", nil), nil)

		require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", body)
		var raw map[string]any
		require.NoError(t, json.Unmarshal(body, &raw))
		assert.Equal(t, "백엔드", raw["title"])
		v, present := raw["scrapInfo"]
		assert.True(t, present)
		assert.Nil(t, v)
	})

	t.Run("리뷰 업스트림 장애 — 200, reviewInfo·firstPageReviews 생략", func(t *testing.T) {
		f := newFakes()
		f.posting.detail = newDetail()
		f.review.scoresErr = context.DeadlineExceeded
		server, _ := newStrictTestServer(t, f)

		resp, body := doRequest(t, http.MethodGet, server.URL+"/api/job_postings/wanted/p1", "", nil)

		require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", body)
		var raw map[string]any
		require.NoError(t, json.Unmarshal(body, &raw))
		assert.Equal(t, "백엔드", raw["title"])
		assert.NotContains(t, raw, "reviewInfo")
		assert.NotContains(t, raw, "firstPageReviews")
	})

	t.Run("동일 공고에 스크랩 다중 매칭 — 500 구조화 (데이터 정합 오류는 열화가 아니라 실패)", func(t *testing.T) {
		f := newFakes()
		f.posting.detail = newDetail()
		f.scrapJob.scrapJobs = []*scrapGrpc.ScrapJob{
			{Site: "wanted", PostingId: "p1", Tags: []string{"a"}},
			{Site: "wanted", PostingId: "p1", Tags: []string{"b"}},
		}
		server, _ := newStrictTestServer(t, f)

		resp, body := doRequest(t, http.MethodGet, server.URL+"/api/job_postings/wanted/p1", newToken(t, "user-1", nil), nil)

		require.Equal(t, http.StatusInternalServerError, resp.StatusCode, "body: %s", body)
		var errResp api.ErrorResponse
		require.NoError(t, json.Unmarshal(body, &errResp))
		assert.Equal(t, "Internal Server Error", errResp.Message)
	})
}

func TestScrapJobListDegradation_ReviewDownIsHardFail(t *testing.T) {
	// 비대칭 고정: 스크랩 목록 조회는 (리뷰 권한 보유 시) 리뷰 점수 실패가 열화가 아니라 500 하드 실패다.
	// 채용공고 목록의 soft-skip과 다른 현행 설계 — 바꾸려면 계약 논의가 선행돼야 한다.
	f := newFakes()
	f.scrapJob.scrapJobs = []*scrapGrpc.ScrapJob{{Site: "wanted", PostingId: "p1"}}
	f.posting.postingsById = fullPosting()
	f.review.scoresErr = context.DeadlineExceeded
	server, _ := newStrictTestServer(t, f)

	token := newToken(t, "user-1", []string{user_authority.AuthorityReadReview})
	resp, body := doRequest(t, http.MethodGet, server.URL+"/api/my/scrap-job", token, nil)

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode, "body: %s", body)
	var errResp api.ErrorResponse
	require.NoError(t, json.Unmarshal(body, &errResp))
	assert.Equal(t, "Internal Server Error", errResp.Message)

	// 권한이 없으면 리뷰 경로 자체를 타지 않아 장애 무관 — 200
	resp, body = doRequest(t, http.MethodGet, server.URL+"/api/my/scrap-job", newToken(t, "user-1", nil), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", body)
}
