package domain

import (
	"time"

	restapi_grpc "github.com/jyoliar-careerhub/careerhub-review-service/api/restapi"
)

type ReviewInfo struct {
	Score           int32  `json:"score"`
	ReviewCount     int32  `json:"reviewCount"`
	DefaultName     string `json:"defaultName"`
	IsCompleteCrawl bool   `json:"isCompleteCrawl"`
}

type Review struct {
	Score            int32     `json:"score"`
	Summary          string    `json:"summary"`
	EmploymentStatus bool      `json:"employmentStatus"`
	ReviewUserId     string    `json:"reviewUserId"`
	JobType          string    `json:"jobType"`
	Date             time.Time `json:"date"`
}

func ConvertGrpcToReviewInfo(reviewInfo *restapi_grpc.CompanyScore) *ReviewInfo {
	return &ReviewInfo{
		Score:           reviewInfo.Score,
		ReviewCount:     reviewInfo.ReviewCount,
		DefaultName:     reviewInfo.CompanyName,
		IsCompleteCrawl: reviewInfo.IsCompleteCrawl,
	}
}

func ConvertGrpcToReviews(reviews []*restapi_grpc.Review) []*Review {
	convertedReviews := make([]*Review, 0, len(reviews))
	for _, review := range reviews {
		convertedReviews = append(convertedReviews, ConvertGrpcToReview(review))
	}
	return convertedReviews
}

func ConvertGrpcToReview(review *restapi_grpc.Review) *Review {
	return &Review{
		Score:            review.Score,
		Summary:          review.Summary,
		EmploymentStatus: review.EmploymentStatus,
		ReviewUserId:     review.ReviewUserId,
		JobType:          review.JobType,
		Date:             time.UnixMilli(review.UnixMilli),
	}
}
