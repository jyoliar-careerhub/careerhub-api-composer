package review

import (
	"context"

	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/common/domain"
	reviewGrpc "github.com/jyoliar-careerhub/careerhub-review-service/api/restapi"
)

const (
	InitPage    = 0
	DefaultSize = 10
)

type ReviewService struct {
	reviewClient reviewGrpc.ReviewReaderGrpcClient
}

func NewReviewService(reviewClient reviewGrpc.ReviewReaderGrpcClient) *ReviewService {
	return &ReviewService{
		reviewClient: reviewClient,
	}
}

func (r *ReviewService) GetReviews(ctx context.Context, companyName string, page int, size int) ([]*domain.Review, error) {
	if page < InitPage {
		page = InitPage
	}

	res, err := r.reviewClient.GetCompanyReviews(ctx, &reviewGrpc.GetCompanyReviewsRequest{
		Site:        domain.ReviewSite,
		CompanyName: companyName,
		Offset:      int64(page * size),
		Limit:       int64(size),
	})

	if err != nil {
		return nil, err
	}

	return domain.ConvertGrpcToReviews(res.Reviews), nil
}
