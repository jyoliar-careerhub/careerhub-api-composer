package domain

import (
	restapi_grpc "github.com/jyoliar-careerhub/careerhub-posting-service/api/restapi"
)

const (
	ReviewSite = "blind"
)

type JobPostingId struct {
	Site      string `json:"site"`
	PostingId string `json:"postingId"`
}
type JobPosting struct {
	Site        string      `json:"site"`
	PostingId   string      `json:"postingId"`
	Title       string      `json:"title"`
	CompanyName string      `json:"companyName"`
	Skills      []string    `json:"skills"`
	Categories  []string    `json:"categories"`
	ImageUrl    *string     `json:"imageUrl"`
	Addresses   []string    `json:"addresses"`
	MinCareer   *int32      `json:"minCareer"`
	MaxCareer   *int32      `json:"maxCareer"`
	Status      string      `json:"status"`
	ScrapInfo   *ScrapInfo  `json:"scrapInfo"`
	ReviewInfo  *ReviewInfo `json:"reviewInfo,omitempty"`
}

type ScrapInfo struct {
	IsScrapped bool     `json:"isScrapped"`
	Tags       []string `json:"tags"`
}

func ConvertGrpcToJobPostingRes(jobPosting *restapi_grpc.JobPostingRes) *JobPosting {
	return &JobPosting{
		Site:        jobPosting.Site,
		PostingId:   jobPosting.PostingId,
		Title:       jobPosting.Title,
		CompanyName: jobPosting.CompanyName,
		Skills:      jobPosting.Skills,
		Categories:  jobPosting.Categories,
		ImageUrl:    jobPosting.ImageUrl,
		Addresses:   jobPosting.Addresses,
		MinCareer:   jobPosting.MinCareer,
		MaxCareer:   jobPosting.MaxCareer,
		Status:      jobPosting.Status,
	}
}

func ConvertGrpcToJobPostingResList(jobPostings []*restapi_grpc.JobPostingRes) []*JobPosting {
	jobPostingResList := make([]*JobPosting, len(jobPostings))
	for i, jobPosting := range jobPostings {
		jobPostingResList[i] = ConvertGrpcToJobPostingRes(jobPosting)
	}
	return jobPostingResList
}

func AttachScrapped(jobPostings []*JobPosting, scrapJobs []*ScrapJob) {
	scrapJobMap := make(map[string]*ScrapJob)
	for _, scrapJob := range scrapJobs {
		scrapJobMap[scrapJob.Site+scrapJob.PostingId] = scrapJob
	}

	for _, jobPosting := range jobPostings {
		if scrapJob, ok := scrapJobMap[jobPosting.Site+jobPosting.PostingId]; ok {
			tags := scrapJob.Tags
			if tags == nil {
				tags = []string{}
			}
			jobPosting.ScrapInfo = &ScrapInfo{
				IsScrapped: true,
				Tags:       tags,
			}
		} else {
			jobPosting.ScrapInfo = &ScrapInfo{
				IsScrapped: false,
				Tags:       []string{},
			}
		}
	}
}

func GetJobPostingIds(jobPostings []*JobPosting) []*JobPostingId {
	jobPostingIds := make([]*JobPostingId, len(jobPostings))
	for i, jobPosting := range jobPostings {
		jobPostingIds[i] = &JobPostingId{
			Site:      jobPosting.Site,
			PostingId: jobPosting.PostingId,
		}
	}
	return jobPostingIds
}

func GetCompanyNames(jobPostings []*JobPosting) []string {
	companyNames := make([]string, len(jobPostings))
	for i, jobPosting := range jobPostings {
		companyNames[i] = jobPosting.CompanyName
	}
	return companyNames
}

type JobPostingDetail struct {
	Site             string      `json:"site"`
	PostingId        string      `json:"postingId"`
	Title            string      `json:"title"`
	Skills           []string    `json:"skills"`
	Intro            string      `json:"intro"`
	MainTask         string      `json:"mainTask"`
	Qualifications   string      `json:"qualifications"`
	Preferred        string      `json:"preferred"`
	Benefits         string      `json:"benefits"`
	RecruitProcess   *string     `json:"recruitProcess"`
	CareerMin        *int32      `json:"careerMin"`
	CareerMax        *int32      `json:"careerMax"`
	Addresses        []string    `json:"addresses"`
	CompanyId        string      `json:"companyId"`
	CompanyName      string      `json:"companyName"`
	CompanyImages    []string    `json:"companyImages"`
	Tags             []string    `json:"tags"`
	Status           string      `json:"status"`
	PostUrl          string      `json:"postUrl"`
	CompanyUrl       *string     `json:"companyUrl"`
	CompanyLogo      string      `json:"companyLogo"`
	ScrapInfo        *ScrapInfo  `json:"scrapInfo"`
	ReviewInfo       *ReviewInfo `json:"reviewInfo,omitempty"`
	FirstPageReviews []*Review   `json:"firstPageReviews,omitempty"`
}

func ConvertGrpcToJobPostingDetail(jobPosting *restapi_grpc.JobPostingDetailRes) *JobPostingDetail {
	return &JobPostingDetail{
		Site:           jobPosting.Site,
		PostingId:      jobPosting.PostingId,
		Title:          jobPosting.Title,
		Skills:         jobPosting.Skills,
		Intro:          jobPosting.Intro,
		MainTask:       jobPosting.MainTask,
		Qualifications: jobPosting.Qualifications,
		Preferred:      jobPosting.Preferred,
		Benefits:       jobPosting.Benefits,
		RecruitProcess: jobPosting.RecruitProcess,
		CareerMin:      jobPosting.CareerMin,
		CareerMax:      jobPosting.CareerMax,
		Addresses:      jobPosting.Addresses,
		CompanyId:      jobPosting.CompanyId,
		CompanyName:    jobPosting.CompanyName,
		CompanyImages:  jobPosting.CompanyImages,
		Tags:           jobPosting.Tags,
		Status:         jobPosting.Status,
		PostUrl:        jobPosting.PostUrl,
		CompanyUrl:     jobPosting.CompanyUrl,
		CompanyLogo:    jobPosting.CompanyLogo,
	}
}
