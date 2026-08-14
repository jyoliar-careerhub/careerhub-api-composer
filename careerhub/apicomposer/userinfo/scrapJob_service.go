package userinfo

import (
	"context"

	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/common/domain"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/common/user_authority"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/jwtresolver"
	postingGrpc "github.com/jyoliar-careerhub/careerhub-posting-service/api/restapi"
	reviewGrpc "github.com/jyoliar-careerhub/careerhub-review-service/api/restapi"
	scrapJobGrpc "github.com/jyoliar-careerhub/careerhub-userinfo-service/api/restapi"
)

type ScrapJobService struct {
	scrapjobClient scrapJobGrpc.ScrapJobGrpcClient
	postingClient  postingGrpc.RestApiGrpcClient
	reviewClient   reviewGrpc.ReviewReaderGrpcClient
}

func NewScrapJobService(
	scrapjobClient scrapJobGrpc.ScrapJobGrpcClient,
	postingClient postingGrpc.RestApiGrpcClient,
	reviewClient reviewGrpc.ReviewReaderGrpcClient,
) *ScrapJobService {
	return &ScrapJobService{
		scrapjobClient: scrapjobClient,
		postingClient:  postingClient,
		reviewClient:   reviewClient,
	}
}

func (s *ScrapJobService) GetScrapJobs(ctx context.Context, claims *jwtresolver.CustomClaims, tag *string) ([]*domain.JobPosting, error) {
	scrapJobRes, err := s.scrapjobClient.GetScrapJobs(ctx, &scrapJobGrpc.GetScrapJobsRequest{
		UserId: claims.UserId,
		Tag:    tag,
	})

	if err != nil {
		return nil, err
	}
	scrapJobs := domain.ConvertGrpcToScrapJobs(scrapJobRes.ScrapJobs)

	jobPostings, err := s.getJobPostingsByPostingIds(ctx, scrapJobs)

	if err != nil {
		return nil, err
	}

	domain.AttachScrapped(jobPostings, scrapJobs)

	if claims.HasAuthority(user_authority.AuthorityReadReview) {
		companyScores, err := s.getReviewScoresByCompanyNames(ctx, domain.GetCompanyNames(jobPostings))
		if err != nil {
			return nil, err
		}

		for _, jobPosting := range jobPostings {
			if companyScore, ok := companyScores[jobPosting.CompanyName]; ok {
				jobPosting.ReviewInfo = &domain.ReviewInfo{
					Score:       companyScore.Score,
					ReviewCount: companyScore.ReviewCount,
					DefaultName: companyScore.CompanyName,
				}
			}
		}
	}

	return jobPostings, nil
}
func (s *ScrapJobService) getJobPostingsByPostingIds(ctx context.Context, scrapJobs []*domain.ScrapJob) ([]*domain.JobPosting, error) {
	postingIds := make([]*postingGrpc.JobPostingIdReq, len(scrapJobs))
	for i, scrapJob := range scrapJobs {
		postingIds[i] = &postingGrpc.JobPostingIdReq{
			Site:      scrapJob.Site,
			PostingId: scrapJob.PostingId,
		}
	}

	postings, err := s.postingClient.JobPostingsById(ctx, &postingGrpc.JobPostingsByIdRequest{
		JobPostingIds: postingIds,
	})

	if err != nil {
		return nil, err
	}

	jobPostings := domain.ConvertGrpcToJobPostingResList(postings.JobPostings)

	return jobPostings, nil
}

func (s *ScrapJobService) getReviewScoresByCompanyNames(ctx context.Context, companyNames []string) (map[string]*reviewGrpc.CompanyScore, error) {
	res, err := s.reviewClient.GetCompanyScores(ctx, &reviewGrpc.GetCompanyScoresRequest{Site: domain.ReviewSite, CompanyNames: companyNames})
	if err != nil {
		return nil, err
	}

	return res.CompanyScores, nil
}

func (s *ScrapJobService) AddScrapJob(ctx context.Context, in *scrapJobGrpc.AddScrapJobRequest) error {
	_, err := s.scrapjobClient.AddScrapJob(ctx, in)
	return err
}
func (s *ScrapJobService) RemoveScrapJob(ctx context.Context, in *scrapJobGrpc.RemoveScrapJobRequest) (bool, error) {
	isExisted, err := s.scrapjobClient.RemoveScrapJob(ctx, in)

	return isExisted.IsExisted, err
}
func (s *ScrapJobService) AddTag(ctx context.Context, in *scrapJobGrpc.AddTagRequest) (bool, error) {
	isExisted, err := s.scrapjobClient.AddTag(ctx, in)

	return isExisted.IsExisted, err
}
func (s *ScrapJobService) RemoveTag(ctx context.Context, in *scrapJobGrpc.RemoveTagRequest) (bool, error) {
	isExisted, err := s.scrapjobClient.RemoveTag(ctx, in)

	return isExisted.IsExisted, err
}
func (s *ScrapJobService) GetScrapTags(ctx context.Context, userId string) (*scrapJobGrpc.GetScrapTagsResponse, error) {
	return s.scrapjobClient.GetScrapTags(ctx, &scrapJobGrpc.GetScrapTagsRequest{
		UserId: userId,
	})
}

func (s *ScrapJobService) GetUntaggedScrapJobs(ctx context.Context, userId string) ([]*scrapJobGrpc.ScrapJob, error) {
	scrapJobRes, err := s.scrapjobClient.GetUntaggedScrapJobs(ctx, &scrapJobGrpc.GetUntaggedScrapJobsRequest{
		UserId: userId,
	})

	if err != nil {
		return nil, err
	}

	return scrapJobRes.ScrapJobs, nil
}
