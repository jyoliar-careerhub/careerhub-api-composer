package controller

import (
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/common/domain"
)

// 도메인 ↔ 생성 타입 변환 모음 (전환 패턴 2단계의 convert*ToAPI).
// 원칙: 성공 응답의 wire 형태를 기존 핸들러(도메인 구조체 직렬화)와 동일하게 유지한다 —
// nil 슬라이스는 nil 포인터로(→ JSON null), 값은 그대로.

// sliceOrNil는 nil 슬라이스를 nil 포인터로 보존한다 — required+nullable 필드의 null 유지.
func sliceOrNil[T any](s []T) *[]T {
	if s == nil {
		return nil
	}
	return &s
}

// sliceOrOmit는 빈 슬라이스를 nil 포인터로 접는다 — proto omitempty(빈 값 필드 생략) 유지.
func sliceOrOmit[T any](s []T) *[]T {
	if len(s) == 0 {
		return nil
	}
	return &s
}

// strOrOmit는 빈 문자열을 nil 포인터로 접는다 — proto omitempty 유지.
func strOrOmit(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func convertScrapInfoToAPI(si *domain.ScrapInfo) *api.ScrapInfo {
	if si == nil {
		return nil
	}
	return &api.ScrapInfo{IsScrapped: si.IsScrapped, Tags: si.Tags}
}

func convertReviewInfoToAPI(ri *domain.ReviewInfo) *api.ReviewInfo {
	if ri == nil {
		return nil
	}
	return &api.ReviewInfo{
		Score:           ri.Score,
		ReviewCount:     ri.ReviewCount,
		DefaultName:     ri.DefaultName,
		IsCompleteCrawl: ri.IsCompleteCrawl,
	}
}

func convertJobPostingToAPI(jp *domain.JobPosting) api.JobPosting {
	return api.JobPosting{
		Site:        jp.Site,
		PostingId:   jp.PostingId,
		Title:       jp.Title,
		CompanyName: jp.CompanyName,
		Skills:      sliceOrNil(jp.Skills),
		Categories:  sliceOrNil(jp.Categories),
		ImageUrl:    jp.ImageUrl,
		Addresses:   sliceOrNil(jp.Addresses),
		MinCareer:   jp.MinCareer,
		MaxCareer:   jp.MaxCareer,
		Status:      api.JobPostingStatus(jp.Status),
		ScrapInfo:   convertScrapInfoToAPI(jp.ScrapInfo),
		ReviewInfo:  convertReviewInfoToAPI(jp.ReviewInfo),
	}
}

func convertJobPostingsToAPI(jobPostings []*domain.JobPosting) []api.JobPosting {
	converted := make([]api.JobPosting, len(jobPostings))
	for i, jp := range jobPostings {
		converted[i] = convertJobPostingToAPI(jp)
	}
	return converted
}

func convertJobPostingDetailToAPI(d *domain.JobPostingDetail) api.JobPostingDetail {
	var firstPageReviews *[]api.Review
	if d.FirstPageReviews != nil {
		reviews := convertReviewsToAPI(d.FirstPageReviews)
		firstPageReviews = &reviews
	}

	return api.JobPostingDetail{
		Site:             d.Site,
		PostingId:        d.PostingId,
		Title:            d.Title,
		Skills:           sliceOrNil(d.Skills),
		Intro:            d.Intro,
		MainTask:         d.MainTask,
		Qualifications:   d.Qualifications,
		Preferred:        d.Preferred,
		Benefits:         d.Benefits,
		RecruitProcess:   d.RecruitProcess,
		CareerMin:        d.CareerMin,
		CareerMax:        d.CareerMax,
		Addresses:        sliceOrNil(d.Addresses),
		CompanyId:        d.CompanyId,
		CompanyName:      d.CompanyName,
		CompanyImages:    sliceOrNil(d.CompanyImages),
		Tags:             sliceOrNil(d.Tags),
		Status:           api.JobPostingStatus(d.Status),
		PostUrl:          d.PostUrl,
		CompanyUrl:       d.CompanyUrl,
		CompanyLogo:      d.CompanyLogo,
		ScrapInfo:        convertScrapInfoToAPI(d.ScrapInfo),
		ReviewInfo:       convertReviewInfoToAPI(d.ReviewInfo),
		FirstPageReviews: firstPageReviews,
	}
}
