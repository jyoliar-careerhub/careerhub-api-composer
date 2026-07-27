package controller

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"
	postingGrpc "github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/posting/restapi_grpc"
	scrapGrpc "github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/userinfo/restapi_grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScrapJobStrict_GetScrapJobs(t *testing.T) {
	f := newFakes()
	f.scrapJob.scrapJobs = []*scrapGrpc.ScrapJob{{Site: "wanted", PostingId: "p1", Tags: []string{"백엔드"}}}
	f.posting.postingsById = []*postingGrpc.JobPostingRes{{Site: "wanted", PostingId: "p1", Title: "백엔드 채용", CompanyName: "acme", Status: "hiring"}}
	server, _ := newStrictTestServer(t, f)

	resp, body := doRequest(t, http.MethodGet, server.URL+"/api/my/scrap-job?tag=백엔드", newToken(t, "user-1", nil), nil)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")

	var postings []api.JobPosting
	require.NoError(t, json.Unmarshal(body, &postings))
	require.Len(t, postings, 1)
	assert.Equal(t, "백엔드 채용", postings[0].Title)
	// 스크랩 목록이므로 scrapInfo가 항상 부착된다 (성공 계약 불변)
	require.NotNil(t, postings[0].ScrapInfo)
	assert.True(t, postings[0].ScrapInfo.IsScrapped)
	assert.Equal(t, []string{"백엔드"}, postings[0].ScrapInfo.Tags)

	// tag 파라미터·토큰 userId가 업스트림 요청으로 전달된다
	require.NotNil(t, f.scrapJob.gotGet)
	assert.Equal(t, "user-1", f.scrapJob.gotGet.UserId)
	require.NotNil(t, f.scrapJob.gotGet.Tag)
	assert.Equal(t, "백엔드", *f.scrapJob.gotGet.Tag)
}

func TestScrapJobStrict_BodyOperations(t *testing.T) {
	t.Run("POST 스크랩 추가 — 201, userId는 토큰에서", func(t *testing.T) {
		f := newFakes()
		server, _ := newStrictTestServer(t, f)

		// 본문의 userId는 명세에 없는 필드 — 무시되고 토큰의 userId가 쓰인다
		resp, body := doRequest(t, http.MethodPost, server.URL+"/api/my/scrap-job", newToken(t, "token-user", nil),
			map[string]string{"site": "wanted", "postingId": "p1", "userId": "attacker"})

		require.Equal(t, http.StatusCreated, resp.StatusCode, "body: %s", body)
		assert.Empty(t, body)
		require.NotNil(t, f.scrapJob.gotAdd)
		assert.Equal(t, "token-user", f.scrapJob.gotAdd.UserId)
		assert.Equal(t, "wanted", f.scrapJob.gotAdd.Site)
		assert.Equal(t, "p1", f.scrapJob.gotAdd.PostingId)
	})

	t.Run("DELETE 스크랩 제거 — 존재하면 200, 없으면 404 구조화", func(t *testing.T) {
		f := newFakes()
		f.scrapJob.isExisted = false
		server, _ := newStrictTestServer(t, f)

		resp, body := doRequest(t, http.MethodDelete, server.URL+"/api/my/scrap-job", newToken(t, "user-1", nil),
			map[string]string{"site": "wanted", "postingId": "p1"})

		require.Equal(t, http.StatusNotFound, resp.StatusCode)
		var errResp api.ErrorResponse
		require.NoError(t, json.Unmarshal(body, &errResp))
		assert.Equal(t, "Scrap job not found", errResp.Message)
	})

	t.Run("태그 추가·제거 — 200 / 404", func(t *testing.T) {
		f := newFakes()
		f.scrapJob.isExisted = true
		server, _ := newStrictTestServer(t, f)

		resp, body := doRequest(t, http.MethodPost, server.URL+"/api/my/scrap-job/tags", newToken(t, "user-1", nil),
			map[string]string{"site": "wanted", "postingId": "p1", "tag": "관심"})
		require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", body)
		require.NotNil(t, f.scrapJob.gotAddTag)
		assert.Equal(t, "관심", f.scrapJob.gotAddTag.Tag)

		resp, _ = doRequest(t, http.MethodDelete, server.URL+"/api/my/scrap-job/tags", newToken(t, "user-1", nil),
			map[string]string{"site": "wanted", "postingId": "p1", "tag": "관심"})
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("본문 형식 불량 — 400 구조화 (기존 500에서 에러 계약 개정)", func(t *testing.T) {
		f := newFakes()
		server, _ := newStrictTestServer(t, f)

		resp, body := doRequest(t, http.MethodPost, server.URL+"/api/my/scrap-job", newToken(t, "user-1", nil), "{broken json")

		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		var errResp api.ErrorResponse
		require.NoError(t, json.Unmarshal(body, &errResp), "body: %s", body)
		assert.NotEmpty(t, errResp.Message)
		assert.Nil(t, f.scrapJob.gotAdd)
	})

	t.Run("필수 본문 필드 누락 — 400 (명세 required)", func(t *testing.T) {
		f := newFakes()
		server, _ := newStrictTestServer(t, f)

		resp, body := doRequest(t, http.MethodPost, server.URL+"/api/my/scrap-job", newToken(t, "user-1", nil),
			map[string]string{"site": "wanted"}) // postingId 누락

		require.Equal(t, http.StatusBadRequest, resp.StatusCode, "body: %s", body)
		var errResp api.ErrorResponse
		require.NoError(t, json.Unmarshal(body, &errResp))
		assert.NotEmpty(t, errResp.Message)
	})
}

func TestScrapJobStrict_UntaggedAndTags(t *testing.T) {
	t.Run("untagged — 스크랩 없으면 null 배열 (성공 계약 불변)", func(t *testing.T) {
		f := newFakes()
		f.scrapJob.scrapJobs = nil
		server, _ := newStrictTestServer(t, f)

		resp, body := doRequest(t, http.MethodGet, server.URL+"/api/my/scrap-job/untagged", newToken(t, "user-1", nil), nil)

		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "null", string(bytesTrim(body)))
	})

	t.Run("untagged — 항목의 빈 필드는 생략 (proto omitempty 유지)", func(t *testing.T) {
		f := newFakes()
		f.scrapJob.scrapJobs = []*scrapGrpc.ScrapJob{{Site: "wanted", PostingId: "p1"}} // Tags 없음
		server, _ := newStrictTestServer(t, f)

		resp, body := doRequest(t, http.MethodGet, server.URL+"/api/my/scrap-job/untagged", newToken(t, "user-1", nil), nil)

		require.Equal(t, http.StatusOK, resp.StatusCode)
		var raw []map[string]any
		require.NoError(t, json.Unmarshal(body, &raw))
		require.Len(t, raw, 1)
		assert.NotContains(t, raw[0], "tags")
	})

	t.Run("tags — 태그 없으면 빈 객체 (proto omitempty 유지)", func(t *testing.T) {
		f := newFakes()
		f.scrapJob.tags = nil
		server, _ := newStrictTestServer(t, f)

		resp, body := doRequest(t, http.MethodGet, server.URL+"/api/my/scrap-job/tags", newToken(t, "user-1", nil), nil)

		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.JSONEq(t, `{}`, string(body))
	})
}

func TestScrapJobStrict_Unauthorized(t *testing.T) {
	// 비로그인 — 핸들러 발신 401은 구조화 JSON (기존 text/plain "Unauthorized"에서 개정)
	f := newFakes()
	server, _ := newStrictTestServer(t, f)

	resp, body := doRequest(t, http.MethodGet, server.URL+"/api/my/scrap-job", "", nil)

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	var errResp api.ErrorResponse
	require.NoError(t, json.Unmarshal(body, &errResp))
	assert.Equal(t, "Unauthorized", errResp.Message)
}

func bytesTrim(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}
