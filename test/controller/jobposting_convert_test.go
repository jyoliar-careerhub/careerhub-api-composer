package controller

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	postingGrpc "github.com/jyoliar-careerhub/careerhub-posting-service/api/restapi"
	reviewGrpc "github.com/jyoliar-careerhub/careerhub-review-service/api/restapi"
	scrapGrpc "github.com/jyoliar-careerhub/careerhub-userinfo-service/api/restapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 변환 매핑 전수 테스트 — 시나리오 테스트(상태별 채움 여부)와 분업한다:
//   - 최대 채움: 모든 필드를 채워 매핑 누락·뒤바뀜을 응답 JSON 통째 비교로 잡는다
//   - 최소 채움: nullable(null 유지) vs 옵셔널(필드 생략) 의미론을 필드별로 잡는다
// 매핑은 상태 무관(순수 변환)이므로 상태 조합을 곱할 필요는 없다.

func ptr[T any](v T) *T { return &v }

func TestJobPostingConvert_FullyPopulated(t *testing.T) {
	f := newFakes()
	f.posting.jobPostings = []*postingGrpc.JobPostingRes{{
		Site: "wanted", PostingId: "p1", Title: "백엔드 시니어", CompanyName: "acme",
		Skills: []string{"go", "kotlin"}, Categories: []string{"server"},
		ImageUrl: ptr("https://img.example/logo.png"), Addresses: []string{"서울 강남"},
		MinCareer: ptr(int32(3)), MaxCareer: ptr(int32(7)), Status: "hiring",
	}}
	f.scrapJob.scrapJobs = []*scrapGrpc.ScrapJob{{Site: "wanted", PostingId: "p1", Tags: []string{"관심"}}}
	f.review.companyScores = map[string]*reviewGrpc.CompanyScore{
		"acme": {CompanyName: "acme", Score: 4, ReviewCount: 12, IsCompleteCrawl: true},
	}
	server, _ := newStrictTestServer(t, f)

	query := encodeQuery(t, map[string]any{})
	resp, body := doRequest(t, http.MethodGet, server.URL+"/api/job_postings?page=1&size=10&encoded_query="+query, newToken(t, "user-1", nil), nil)

	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", body)
	assert.JSONEq(t, `[{
		"site": "wanted",
		"postingId": "p1",
		"title": "백엔드 시니어",
		"companyName": "acme",
		"skills": ["go", "kotlin"],
		"categories": ["server"],
		"imageUrl": "https://img.example/logo.png",
		"addresses": ["서울 강남"],
		"minCareer": 3,
		"maxCareer": 7,
		"status": "hiring",
		"scrapInfo": {"isScrapped": true, "tags": ["관심"]},
		"reviewInfo": {"score": 4, "reviewCount": 12, "defaultName": "acme", "isCompleteCrawl": true}
	}]`, string(body))
}

func TestJobPostingConvert_MinimallyPopulated(t *testing.T) {
	// 로그인 상태로 요청한다 — 이 테스트의 관심사는 "데이터가 비었을 때의 wire"뿐이다.
	// (비로그인의 scrapInfo null은 상태의 결과라 시나리오 테스트가 전담한다.
	//  로그인 + 스크랩 없음은 null이 아니라 {isScrapped:false, tags:[]}로 나온다 — 여기서 고정.)
	f := newFakes()
	f.posting.jobPostings = []*postingGrpc.JobPostingRes{{
		Site: "wanted", PostingId: "p1", Status: "closed",
		// 나머지 전부 제로값: 문자열 "" · 슬라이스 nil · 포인터 nil
	}}
	server, _ := newStrictTestServer(t, f)

	query := encodeQuery(t, map[string]any{})
	resp, body := doRequest(t, http.MethodGet, server.URL+"/api/job_postings?page=1&size=10&encoded_query="+query, newToken(t, "user-1", nil), nil)

	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", body)
	// required+nullable 필드는 null로 존재, 옵셔널(reviewInfo)은 부재 — JSONEq가 부재까지 강제한다
	assert.JSONEq(t, `[{
		"site": "wanted",
		"postingId": "p1",
		"title": "",
		"companyName": "",
		"skills": null,
		"categories": null,
		"imageUrl": null,
		"addresses": null,
		"minCareer": null,
		"maxCareer": null,
		"status": "closed",
		"scrapInfo": {"isScrapped": false, "tags": []}
	}]`, string(body))
}

func TestJobPostingDetailConvert_FullyPopulated(t *testing.T) {
	reviewMilli := time.Date(2026, 6, 1, 9, 30, 0, 0, time.UTC).UnixMilli()
	f := newFakes()
	f.posting.detail = &postingGrpc.JobPostingDetailRes{
		Site: "wanted", PostingId: "p1", Title: "백엔드 시니어",
		Skills: []string{"go"}, Intro: "소개", MainTask: "주요 업무",
		Qualifications: "자격 요건", Preferred: "우대 사항", Benefits: "복지",
		RecruitProcess: ptr("서류-면접"), CareerMin: ptr(int32(3)), CareerMax: ptr(int32(7)),
		Addresses: []string{"서울 강남"}, CompanyId: "c1", CompanyName: "acme",
		CompanyImages: []string{"https://img.example/office.png"}, Tags: []string{"성장"},
		Status: "hiring", PostUrl: "https://ex.com/p1",
		CompanyUrl: ptr("https://acme.example"), CompanyLogo: "https://img.example/logo.png",
	}
	f.scrapJob.scrapJobs = []*scrapGrpc.ScrapJob{{Site: "wanted", PostingId: "p1", Tags: []string{"관심"}}}
	f.review.companyScores = map[string]*reviewGrpc.CompanyScore{
		"acme": {CompanyName: "acme", Score: 4, ReviewCount: 12, IsCompleteCrawl: true},
	}
	f.review.reviews = []*reviewGrpc.Review{{
		Score: 5, Summary: "좋은 회사", EmploymentStatus: true,
		ReviewUserId: "rev-user", JobType: "백엔드", UnixMilli: reviewMilli,
	}}
	server, _ := newStrictTestServer(t, f)

	resp, body := doRequest(t, http.MethodGet, server.URL+"/api/job_postings/wanted/p1", newToken(t, "user-1", nil), nil)

	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", body)
	// date는 time.UnixMilli(로컬 타임존) 직렬화라 기대값을 동일 방식으로 생성한다 (TZ 무관)
	expectedDate, err := time.UnixMilli(reviewMilli).MarshalJSON()
	require.NoError(t, err)
	assert.JSONEq(t, fmt.Sprintf(`{
		"site": "wanted",
		"postingId": "p1",
		"title": "백엔드 시니어",
		"skills": ["go"],
		"intro": "소개",
		"mainTask": "주요 업무",
		"qualifications": "자격 요건",
		"preferred": "우대 사항",
		"benefits": "복지",
		"recruitProcess": "서류-면접",
		"careerMin": 3,
		"careerMax": 7,
		"addresses": ["서울 강남"],
		"companyId": "c1",
		"companyName": "acme",
		"companyImages": ["https://img.example/office.png"],
		"tags": ["성장"],
		"status": "hiring",
		"postUrl": "https://ex.com/p1",
		"companyUrl": "https://acme.example",
		"companyLogo": "https://img.example/logo.png",
		"scrapInfo": {"isScrapped": true, "tags": ["관심"]},
		"reviewInfo": {"score": 4, "reviewCount": 12, "defaultName": "acme", "isCompleteCrawl": true},
		"firstPageReviews": [{
			"score": 5,
			"summary": "좋은 회사",
			"employmentStatus": true,
			"reviewUserId": "rev-user",
			"jobType": "백엔드",
			"date": %s
		}]
	}`, expectedDate), string(body))
}

func TestJobPostingDetailConvert_MinimallyPopulated(t *testing.T) {
	// 로그인 + 스크랩 없음 — 목록 min 테스트와 같은 축 분리 (scrapInfo는 null이 아니라 빈 형태)
	f := newFakes()
	f.posting.detail = &postingGrpc.JobPostingDetailRes{
		Site: "wanted", PostingId: "p1", Status: "closed",
		// 나머지 전부 제로값
	}
	server, _ := newStrictTestServer(t, f)

	resp, body := doRequest(t, http.MethodGet, server.URL+"/api/job_postings/wanted/p1", newToken(t, "user-1", nil), nil)

	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", body)
	// nullable은 null로 존재, 옵셔널(reviewInfo·firstPageReviews)은 부재
	assert.JSONEq(t, `{
		"site": "wanted",
		"postingId": "p1",
		"title": "",
		"skills": null,
		"intro": "",
		"mainTask": "",
		"qualifications": "",
		"preferred": "",
		"benefits": "",
		"recruitProcess": null,
		"careerMin": null,
		"careerMax": null,
		"addresses": null,
		"companyId": "",
		"companyName": "",
		"companyImages": null,
		"tags": null,
		"status": "closed",
		"postUrl": "",
		"companyUrl": null,
		"companyLogo": "",
		"scrapInfo": {"isScrapped": false, "tags": []}
	}`, string(body))
}
