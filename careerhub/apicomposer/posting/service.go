package posting

import (
	"context"
	"fmt"

	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/common/domain"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/common/query"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/jwtresolver"
	postingGrpc "github.com/jyoliar-careerhub/careerhub-posting-service/api/restapi"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/review"
	reviewGrpc "github.com/jyoliar-careerhub/careerhub-review-service/api/restapi"
	scrapGrpc "github.com/jyoliar-careerhub/careerhub-userinfo-service/api/restapi"
	"github.com/jae2274/goutils/cchan/async"
	"github.com/jae2274/goutils/llog"
	"github.com/jae2274/goutils/optional"
	"google.golang.org/protobuf/types/known/emptypb"
)

type PostingService struct {
	postingClient  postingGrpc.RestApiGrpcClient
	scrapjobClient scrapGrpc.ScrapJobGrpcClient
	reviewClient   reviewGrpc.ReviewReaderGrpcClient
}

func NewPostingService(postingClient postingGrpc.RestApiGrpcClient, scrapjobClient scrapGrpc.ScrapJobGrpcClient, reviewClient reviewGrpc.ReviewReaderGrpcClient) *PostingService {
	return &PostingService{
		postingClient:  postingClient,
		scrapjobClient: scrapjobClient,
		reviewClient:   reviewClient,
	}
}

func (p *PostingService) JobPostingsWithClaims(ctx context.Context, req *JobPostingsRequest, claims *jwtresolver.CustomClaims) ([]*domain.JobPosting, error) {
	jobPostings, err := p.JobPostings(ctx, req)
	if err != nil {
		return nil, err
	}

	// var scrapJobsChan <-chan async.Result[[]*domain.ScrapJob]
	// var companyScoresChan <-chan async.Result[*reviewGrpc.GetCompanyScoresResponse]

	// if claims.HasAuthority(user_authority.AuthorityScrapJob) {
	scrapJobsChan := async.ExecAsync(func() ([]*domain.ScrapJob, error) {
		return p.getScrapJobsByPostingIds(ctx, claims.UserId, domain.GetJobPostingIds(jobPostings))
	})
	// }

	// if claims.HasAuthority(user_authority.AuthorityReadReview) {
	// 	companyScoresChan = async.ExecAsync(func() (*reviewGrpc.GetCompanyScoresResponse, error) {
	// 		return p.reviewClient.GetCompanyScores(ctx, &reviewGrpc.GetCompanyScoresRequest{Site: domain.ReviewSite, CompanyNames: domain.GetCompanyNames(jobPostings)})
	// 	})
	// }

	if scrapJobsChan != nil {
		scrapJobsResult := <-scrapJobsChan

		scrapJobs := scrapJobsResult.Value
		err := scrapJobsResult.Err

		if err != nil { // if error occurs, just log it and continue
			llog.LogErr(ctx, err)
		} else {
			domain.AttachScrapped(jobPostings, scrapJobs)
		}
	}

	// attachCompanyScores(jobPostings, companyScoresChan)

	return jobPostings, nil
}

func attachCompanyScores(ctx context.Context, jobPostings []*domain.JobPosting, companyScoresChan <-chan async.Result[*reviewGrpc.GetCompanyScoresResponse]) {
	if companyScoresChan != nil {
		companyScoresResult := <-companyScoresChan

		companyScores := companyScoresResult.Value
		err := companyScoresResult.Err

		if err != nil { // if error occurs, just log it and continue
			llog.LogErr(ctx, err)
		} else {
			for _, jobPosting := range jobPostings {
				if companyScore, ok := companyScores.CompanyScores[jobPosting.CompanyName]; ok {
					jobPosting.ReviewInfo = domain.ConvertGrpcToReviewInfo(companyScore)
				}
			}
		}
	}
}

func convertQueryReqToGrpc(queryReq *query.Query) *postingGrpc.QueryReq {
	pbCategories := make([]*postingGrpc.CategoryQueryReq, len(queryReq.Categories))
	for i, category := range queryReq.Categories {
		pbCategories[i] = &postingGrpc.CategoryQueryReq{
			Site:         category.Site,
			CategoryName: category.CategoryName,
		}
	}

	pbSkillNames := make([]*postingGrpc.SkillQueryReq, len(queryReq.SkillNames))
	for i, skillNames := range queryReq.SkillNames {
		pbSkillNames[i] = &postingGrpc.SkillQueryReq{
			Or: skillNames,
		}
	}

	pbCompanies := make([]*postingGrpc.SiteCompanyQueryReq, 0, len(queryReq.Companies))
	for _, company := range queryReq.Companies {
		pbCompanies = append(pbCompanies, &postingGrpc.SiteCompanyQueryReq{
			Site:        company.Site,
			CompanyName: company.CompanyName,
		})
	}

	return &postingGrpc.QueryReq{
		Categories: pbCategories,
		SkillNames: pbSkillNames,
		MinCareer:  queryReq.MinCareer,
		MaxCareer:  queryReq.MaxCareer,
		Companies:  pbCompanies,
	}
}

func (p *PostingService) JobPostings(ctx context.Context, req *JobPostingsRequest) ([]*domain.JobPosting, error) {

	jobPostings, err := p.postingClient.JobPostings(ctx, &postingGrpc.JobPostingsRequest{
		Page:     req.Page - 1,
		Size:     req.Size,
		QueryReq: convertQueryReqToGrpc(req.QueryReq),
	})

	if err != nil {
		return nil, err
	}

	jobPostingResList := domain.ConvertGrpcToJobPostingResList(jobPostings.JobPostings)

	companyScoresChan := async.ExecAsync(func() (*reviewGrpc.GetCompanyScoresResponse, error) {
		return p.reviewClient.GetCompanyScores(ctx, &reviewGrpc.GetCompanyScoresRequest{Site: domain.ReviewSite, CompanyNames: domain.GetCompanyNames(jobPostingResList)})
	})

	attachCompanyScores(ctx, jobPostingResList, companyScoresChan)

	return jobPostingResList, nil
}

func (p *PostingService) CountJobPostings(ctx context.Context, reqQuery *query.Query) (int64, error) {
	res, err := p.postingClient.CountJobPostings(ctx, &postingGrpc.JobPostingsRequest{
		QueryReq: convertQueryReqToGrpc(reqQuery),
	})
	if err != nil {
		return 0, err
	}

	return res.Count, nil
}

func (p *PostingService) getScrapJobsByPostingIds(ctx context.Context, userId string, jobPostings []*domain.JobPostingId) ([]*domain.ScrapJob, error) {
	if len(jobPostings) == 0 {
		return []*domain.ScrapJob{}, nil
	}

	jobPostingIds := make([]*scrapGrpc.JobPostingId, len(jobPostings))
	for i, jobPosting := range jobPostings {
		jobPostingIds[i] = &scrapGrpc.JobPostingId{
			Site:      jobPosting.Site,
			PostingId: jobPosting.PostingId,
		}
	}

	res, err := p.scrapjobClient.GetScrapJobsById(ctx, &scrapGrpc.GetScrapJobsByIdRequest{
		UserId:        userId,
		JobPostingIds: jobPostingIds,
	})
	if err != nil {
		return nil, err
	}

	scrapJobs := make([]*domain.ScrapJob, len(res.ScrapJobs))
	for i, scrapJob := range res.ScrapJobs {
		scrapJobs[i] = domain.ConvertGrpcToScrapJob(scrapJob)
	}

	return scrapJobs, nil
}

type CompanyReviewInfo struct {
	*reviewGrpc.CompanyScore
	Reviews []*reviewGrpc.Review
}

func (p *PostingService) JobPostingDetailWithClaims(ctx context.Context, req *postingGrpc.JobPostingDetailRequest, claims *jwtresolver.CustomClaims) (*domain.JobPostingDetail, bool, error) {
	res, isExist, err := p.JobPostingDetail(ctx, req)
	if err != nil || !isExist {
		return nil, isExist, err
	}

	// var scrapJobsChan <-chan async.Result[[]*domain.ScrapJob]
	// var companyReviewInfoChan <-chan async.Result[optional.Optional[CompanyReviewInfo]]

	// if claims.HasAuthority(user_authority.AuthorityScrapJob) {
	scrapJobsChan := async.ExecAsync(func() ([]*domain.ScrapJob, error) {
		return p.getScrapJobsByPostingIds(ctx, claims.UserId, []*domain.JobPostingId{{Site: res.Site, PostingId: res.PostingId}})
	})
	// }

	// if claims.HasAuthority(user_authority.AuthorityReadReview) {
	// 	companyReviewInfoChan = getFirstReviewPage(ctx, res.CompanyName)
	// }

	if scrapJobsChan != nil {
		scrapJobsResult := <-scrapJobsChan

		scrapJobs := scrapJobsResult.Value
		err := scrapJobsResult.Err

		if err != nil {
			llog.LogErr(ctx, err)
		} else {
			if len(scrapJobs) > 1 {
				return nil, false, fmt.Errorf("multiple scrap jobs found for the same job posting id. site: %s, postingId: %s", res.Site, res.PostingId)
			}
			if len(scrapJobs) == 1 {
				tags := scrapJobs[0].Tags
				if tags == nil {
					tags = []string{}
				}
				res.ScrapInfo = &domain.ScrapInfo{
					IsScrapped: true,
					Tags:       tags,
				}
			} else {
				res.ScrapInfo = &domain.ScrapInfo{
					IsScrapped: false,
					Tags:       []string{},
				}
			}
		}
	}

	// attachReviewInfo(ctx, companyReviewInfoChan, res)

	return res, true, nil
}
func (p *PostingService) getFirstReviewPage(ctx context.Context, companyName string) <-chan async.Result[optional.Optional[CompanyReviewInfo]] {
	return async.ExecAsync(func() (optional.Optional[CompanyReviewInfo], error) {
		companyScore, err := p.reviewClient.GetCompanyScores(ctx, &reviewGrpc.GetCompanyScoresRequest{Site: domain.ReviewSite, CompanyNames: []string{companyName}})
		if err != nil {
			return optional.NewEmptyOptional[CompanyReviewInfo](), err
		}

		if companyScore.CompanyScores[companyName] != nil {
			var companyReviewInfo CompanyReviewInfo
			companyReviewInfo.CompanyScore = companyScore.CompanyScores[companyName]

			reviewsRes, err := p.reviewClient.GetCompanyReviews(ctx, &reviewGrpc.GetCompanyReviewsRequest{
				Site:        domain.ReviewSite,
				CompanyName: companyName,
				Offset:      0,
				Limit:       review.DefaultSize,
			})

			if err != nil {
				return optional.NewEmptyOptional[CompanyReviewInfo](), err
			}

			companyReviewInfo.Reviews = reviewsRes.Reviews
			return optional.NewOptional(&companyReviewInfo), nil
		} else {
			return optional.NewEmptyOptional[CompanyReviewInfo](), nil
		}
	})
}

func attachReviewInfo(ctx context.Context, companyReviewInfoChan <-chan async.Result[optional.Optional[CompanyReviewInfo]], res *domain.JobPostingDetail) {
	if companyReviewInfoChan != nil {
		companyScoresResult := <-companyReviewInfoChan

		companyScoresOpt := companyScoresResult.Value
		err := companyScoresResult.Err

		if err != nil {
			llog.LogErr(ctx, err)
		} else {
			if companyScoresOpt.IsPresent() {
				res.ReviewInfo = domain.ConvertGrpcToReviewInfo(companyScoresOpt.GetPtr().CompanyScore)
				res.FirstPageReviews = domain.ConvertGrpcToReviews(companyScoresOpt.GetPtr().Reviews)
			}
		}
	}
}

func (p *PostingService) JobPostingDetail(ctx context.Context, req *postingGrpc.JobPostingDetailRequest) (*domain.JobPostingDetail, bool, error) {
	res, err := p.postingClient.JobPostingDetail(ctx, req)
	if err != nil {
		return nil, false, err
	}

	if !res.IsExist {
		return nil, false, nil
	}

	jobPostingDetail := domain.ConvertGrpcToJobPostingDetail(res.Detail)

	companyReviewInfoChan := p.getFirstReviewPage(ctx, jobPostingDetail.CompanyName)
	attachReviewInfo(ctx, companyReviewInfoChan, jobPostingDetail)
	return jobPostingDetail, true, nil
}

func (p *PostingService) Categories(ctx context.Context) (*postingGrpc.CategoriesResponse, error) {
	return p.postingClient.Categories(ctx, &emptypb.Empty{})
}

func (p *PostingService) Skills(ctx context.Context) (*postingGrpc.SkillsResponse, error) {
	return p.postingClient.Skills(ctx, &emptypb.Empty{})
}

func (p *PostingService) Companies(ctx context.Context, keyword string) (*postingGrpc.CompaniesResponse, error) {
	return p.postingClient.Companies(ctx, &postingGrpc.CompaniesRequest{PrefixKeyword: keyword, Limit: 5})
}
