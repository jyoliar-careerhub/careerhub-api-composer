package domain

import (
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/common/query"
	restapi_grpc "github.com/jyoliar-careerhub/careerhub-userinfo-service/api/restapi"
)

type ScrapJob struct {
	Site      string   `json:"site"`
	PostingId string   `json:"postingId"`
	Tags      []string `json:"tags"`
}

func ConvertGrpcToScrapJob(scrapJob *restapi_grpc.ScrapJob) *ScrapJob {
	return &ScrapJob{
		Site:      scrapJob.Site,
		PostingId: scrapJob.PostingId,
		Tags:      scrapJob.Tags,
	}
}

func ConvertGrpcToScrapJobs(scrapJobs []*restapi_grpc.ScrapJob) []*ScrapJob {
	result := make([]*ScrapJob, len(scrapJobs))
	for i, scrapJob := range scrapJobs {
		result[i] = ConvertGrpcToScrapJob(scrapJob)
	}
	return result
}

type MatchJob struct {
	Conditions  []*Condition `json:"conditions"`
	AgreeToMail bool         `json:"agreeToMail"`
}

type Condition struct {
	ConditionId   string       `json:"conditionId"`
	ConditionName string       `json:"conditionName"`
	Query         *query.Query `json:"query"`
}
