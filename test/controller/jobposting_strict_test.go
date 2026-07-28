package controller

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"
	postingGrpc "github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/posting/restapi_grpc"
	scrapGrpc "github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/userinfo/restapi_grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// encodeQuery — 프론트와 같은 방식으로 검색 조건을 base64(JSON) 인코딩한다.
func encodeQuery(t *testing.T, q map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(q)
	require.NoError(t, err)
	return url.QueryEscape(base64.StdEncoding.EncodeToString(raw))
}

func TestJobPostingStrict_GetJobPostings(t *testing.T) {
	imageUrl := "https://img.example/logo.png"
	newFakePosting := func() *fakePostingClient {
		return &fakePostingClient{jobPostings: []*postingGrpc.JobPostingRes{{
			Site: "wanted", PostingId: "p1", Title: "백엔드", CompanyName: "acme",
			Skills: []string{"go"}, Status: "hiring", ImageUrl: &imageUrl,
		}}}
	}

	t.Run("비로그인 — scrapInfo null (성공 계약 불변)", func(t *testing.T) {
		f := newFakes()
		f.posting = newFakePosting()
		server, _ := newStrictTestServer(t, f)

		query := encodeQuery(t, map[string]any{"categories": [](any){}, "skillNames": [][]string{}, "minCareer": nil, "maxCareer": nil, "companies": [](any){}})
		resp, body := doRequest(t, http.MethodGet, server.URL+"/api/job_postings?page=1&size=10&encoded_query="+query, "", nil)

		require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", body)
		var raw []map[string]any
		require.NoError(t, json.Unmarshal(body, &raw))
		require.Len(t, raw, 1)
		// scrapInfo 필드는 존재하되 null — required+nullable 전사 유지
		v, present := raw[0]["scrapInfo"]
		assert.True(t, present)
		assert.Nil(t, v)
		assert.Equal(t, "hiring", raw[0]["status"])

		// 페이지 변환: 서비스가 page-1을 업스트림에 보낸다 (기존과 동일)
		require.NotNil(t, f.posting.gotJobReq)
		assert.Equal(t, int32(0), f.posting.gotJobReq.Page)
		assert.Equal(t, int32(10), f.posting.gotJobReq.Size)
	})

	t.Run("로그인 — scrapInfo 부착 (옵셔널 인증 유지)", func(t *testing.T) {
		f := newFakes()
		f.posting = newFakePosting()
		f.scrapJob.scrapJobs = []*scrapGrpc.ScrapJob{{Site: "wanted", PostingId: "p1", Tags: []string{"관심"}}}
		server, _ := newStrictTestServer(t, f)

		query := encodeQuery(t, map[string]any{})
		resp, body := doRequest(t, http.MethodGet, server.URL+"/api/job_postings?page=1&size=10&encoded_query="+query, newToken(t, "user-1", nil), nil)

		require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", body)
		var postings []api.JobPosting
		require.NoError(t, json.Unmarshal(body, &postings))
		require.Len(t, postings, 1)
		require.NotNil(t, postings[0].ScrapInfo)
		assert.True(t, postings[0].ScrapInfo.IsScrapped)
		assert.Equal(t, []string{"관심"}, postings[0].ScrapInfo.Tags)
	})

	t.Run("파라미터 형식 위반 — 명세 검증 미들웨어가 400 구조화로 거부", func(t *testing.T) {
		f := newFakes()
		server, _ := newStrictTestServer(t, f)
		query := encodeQuery(t, map[string]any{})

		for name, path := range map[string]string{
			"page=0 (minimum 1)":     "/api/job_postings?page=0&size=10&encoded_query=" + query,
			"size=101 (maximum 100)": "/api/job_postings?page=1&size=101&encoded_query=" + query,
			"page 누락 (required)":     "/api/job_postings?size=10&encoded_query=" + query,
		} {
			resp, body := doRequest(t, http.MethodGet, server.URL+path, "", nil)
			require.Equal(t, http.StatusBadRequest, resp.StatusCode, "%s — body: %s", name, body)
			var errResp api.ErrorResponse
			require.NoError(t, json.Unmarshal(body, &errResp), "%s — body: %s", name, body)
			assert.NotEmpty(t, errResp.Message, name)
		}
	})

	t.Run("encoded_query 위반 — JSON 파싱 실패·교차 검증은 구현이 400으로", func(t *testing.T) {
		f := newFakes()
		server, _ := newStrictTestServer(t, f)

		// base64는 유효하나 JSON이 아님
		badJSON := url.QueryEscape(base64.StdEncoding.EncodeToString([]byte("not-json")))
		resp, body := doRequest(t, http.MethodGet, server.URL+"/api/job_postings?page=1&size=10&encoded_query="+badJSON, "", nil)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		var errResp api.ErrorResponse
		require.NoError(t, json.Unmarshal(body, &errResp))
		assert.Equal(t, "bad request", errResp.Message)

		// minCareer > maxCareer — 명세로 표현 불가한 교차 검증 (IsValid 유지)
		crossQuery := encodeQuery(t, map[string]any{"minCareer": 5, "maxCareer": 3})
		resp, body = doRequest(t, http.MethodGet, server.URL+"/api/job_postings?page=1&size=10&encoded_query="+crossQuery, "", nil)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		require.NoError(t, json.Unmarshal(body, &errResp))
		assert.Contains(t, errResp.Message, "minCareer")
	})
}

func TestJobPostingStrict_CountAndDetail(t *testing.T) {
	t.Run("count", func(t *testing.T) {
		f := newFakes()
		f.posting.count = 42
		server, _ := newStrictTestServer(t, f)

		query := encodeQuery(t, map[string]any{})
		resp, body := doRequest(t, http.MethodGet, server.URL+"/api/job_postings/count?encoded_query="+query, "", nil)

		require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", body)
		assert.JSONEq(t, `{"count": 42}`, string(body))
	})

	t.Run("detail — 경로 파라미터가 생성 바인딩으로 전달된다", func(t *testing.T) {
		f := newFakes()
		f.posting.detail = &postingGrpc.JobPostingDetailRes{
			Site: "wanted", PostingId: "p1", Title: "백엔드", CompanyId: "c1",
			CompanyName: "acme", Status: "hiring", PostUrl: "https://ex.com/p1", CompanyLogo: "logo",
		}
		server, _ := newStrictTestServer(t, f)

		resp, body := doRequest(t, http.MethodGet, server.URL+"/api/job_postings/wanted/p1", "", nil)

		require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", body)
		require.NotNil(t, f.posting.gotDetailReq)
		assert.Equal(t, "wanted", f.posting.gotDetailReq.Site)
		assert.Equal(t, "p1", f.posting.gotDetailReq.PostingId)

		var raw map[string]any
		require.NoError(t, json.Unmarshal(body, &raw))
		assert.Equal(t, "hiring", raw["status"])
		// 비로그인 → scrapInfo null · 리뷰 없음 → reviewInfo/firstPageReviews 생략 (성공 계약 불변)
		v, present := raw["scrapInfo"]
		assert.True(t, present)
		assert.Nil(t, v)
		assert.NotContains(t, raw, "reviewInfo")
		assert.NotContains(t, raw, "firstPageReviews")
	})

	t.Run("detail — 부재 시 404 구조화", func(t *testing.T) {
		f := newFakes()
		f.posting.detail = nil
		server, _ := newStrictTestServer(t, f)

		resp, body := doRequest(t, http.MethodGet, server.URL+"/api/job_postings/wanted/none", "", nil)

		require.Equal(t, http.StatusNotFound, resp.StatusCode)
		var errResp api.ErrorResponse
		require.NoError(t, json.Unmarshal(body, &errResp))
		assert.Equal(t, "job posting not found", errResp.Message)
	})
}

func TestJobPostingStrict_CatalogEndpoints(t *testing.T) {
	t.Run("categories — Cache-Control 유지·빈 값 필드 생략", func(t *testing.T) {
		f := newFakes()
		f.posting.categories = &postingGrpc.CategoriesResponse{CategoriesBySite: []*postingGrpc.CategoryRes{
			{Site: "wanted", Categories: []string{"백엔드", "프론트"}},
		}}
		server, _ := newStrictTestServer(t, f)

		resp, body := doRequest(t, http.MethodGet, server.URL+"/api/categories", "", nil)

		require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", body)
		assert.Equal(t, "max-age=86400, public", resp.Header.Get("Cache-Control"))
		assert.JSONEq(t, `{"categoriesBySite":[{"site":"wanted","categories":["백엔드","프론트"]}]}`, string(body))
	})

	t.Run("skills — 비어 있으면 빈 객체 (proto omitempty 유지)", func(t *testing.T) {
		f := newFakes()
		f.posting.skills = &postingGrpc.SkillsResponse{}
		server, _ := newStrictTestServer(t, f)

		resp, body := doRequest(t, http.MethodGet, server.URL+"/api/skills", "", nil)

		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "max-age=86400, public", resp.Header.Get("Cache-Control"))
		assert.JSONEq(t, `{}`, string(body))
	})

	t.Run("companies — keyword 필수·Cache-Control 유지", func(t *testing.T) {
		f := newFakes()
		f.posting.companies = &postingGrpc.CompaniesResponse{Companies: []*postingGrpc.CompanyRes{
			{DefaultName: "acme", SiteCompanies: []*postingGrpc.SiteCompanyRes{{Site: "wanted", CompanyId: "c1", CompanyName: "acme"}}},
		}}
		server, _ := newStrictTestServer(t, f)

		resp, body := doRequest(t, http.MethodGet, server.URL+"/api/companies?keyword=ac", "", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", body)
		assert.Equal(t, "max-age=43200, public", resp.Header.Get("Cache-Control"))
		require.NotNil(t, f.posting.gotCompanies)
		assert.Equal(t, "ac", f.posting.gotCompanies.PrefixKeyword)
		assert.Equal(t, int64(5), f.posting.gotCompanies.Limit)

		// keyword 빈 값 — 명세 minLength로 400
		resp, body = doRequest(t, http.MethodGet, server.URL+"/api/companies?keyword=", "", nil)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode, "body: %s", body)
		var errResp api.ErrorResponse
		require.NoError(t, json.Unmarshal(body, &errResp))
		assert.NotEmpty(t, errResp.Message)
	})
}
