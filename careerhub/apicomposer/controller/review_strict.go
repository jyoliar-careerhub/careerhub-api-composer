package controller

import (
	"context"

	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/common/domain"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/review"
	"github.com/jae2274/goutils/llog"
)

// ReviewStrictController는 명세(api/openapi.yaml)에서 생성된 strict 인터페이스로
// review 엔드포인트를 서빙한다. 파라미터 파싱·응답 인코딩은 생성 코드(api.gen.go)가 하고,
// 여기에는 검증·서비스 호출·타입 변환만 남는다.
//
// 전환 패턴 — 이후 컨트롤러 전환은 이 파일을 따라한다:
//  1. 명세에서 해당 엔드포인트의 에러 응답을 ErrorResponse(application/json)로 개정하고 make openapi
//  2. <이름>_strict.go에 RequestObject → ResponseObject 시그니처의 메서드를 구현
//     (도메인 타입 ↔ 생성 타입 변환은 convert*ToAPI 함수로 분리)
//  3. 메서드 시그니처 구속 선언(아래 var _)으로 명세-구현 정합을 컴파일에 맡긴다
//  4. app.go 배선은 하지 않는다 — 전 컨트롤러 구현 완료 후 일괄 전환한다
//     (ops-workflow#14 코멘트의 사람 결정. 배선 방법은 NewStrictHTTPHandler 참고)
type ReviewStrictController struct {
	reviewService *review.ReviewService
}

func NewReviewStrictController(reviewService *review.ReviewService) *ReviewStrictController {
	return &ReviewStrictController{reviewService: reviewService}
}

// 시그니처 구속: 명세가 바뀌어 생성 타입이 달라지면 여기서 컴파일이 깨진다.
// 전 메서드 구현이 끝나는 일괄 전환 시점에 var _ api.StrictServerInterface = ... 단언으로 대체한다.
var _ func(context.Context, api.GetCompanyReviewsRequestObject) (api.GetCompanyReviewsResponseObject, error) = (&ReviewStrictController{}).GetCompanyReviews

func (c *ReviewStrictController) GetCompanyReviews(ctx context.Context, request api.GetCompanyReviewsRequestObject) (api.GetCompanyReviewsResponseObject, error) {
	params := request.Params

	// 검증·기본값은 기존 핸들러(review_controller.go)와 동작 동일하게 유지한다 — 성공 계약 불변
	page := review.InitPage
	if params.Page != nil {
		if *params.Page < review.InitPage {
			return api.GetCompanyReviews400JSONResponse{Message: "Invalid page"}, nil
		}
		page = *params.Page
	}

	size := review.DefaultSize
	if params.Size != nil {
		if *params.Size < 0 {
			return api.GetCompanyReviews400JSONResponse{Message: "Invalid page size"}, nil
		}
		size = *params.Size
	}

	if params.CompanyName == "" {
		return api.GetCompanyReviews400JSONResponse{Message: "companyName is required"}, nil
	}

	reviews, err := c.reviewService.GetReviews(ctx, params.CompanyName, page, size)
	if err != nil {
		// 오류 원문은 로그로만 — 응답에는 노출하지 않는다 (기존 err.Error() 노출은 에러 계약 개정으로 폐기)
		llog.LogErr(ctx, err)
		return api.GetCompanyReviews500JSONResponse{Message: "Internal Server Error"}, nil
	}

	return api.GetCompanyReviews200JSONResponse(convertReviewsToAPI(reviews)), nil
}

func convertReviewsToAPI(reviews []*domain.Review) []api.Review {
	converted := make([]api.Review, len(reviews))
	for i, r := range reviews {
		converted[i] = api.Review{
			Score:            r.Score,
			Summary:          r.Summary,
			EmploymentStatus: r.EmploymentStatus,
			ReviewUserId:     r.ReviewUserId,
			JobType:          r.JobType,
			Date:             r.Date,
		}
	}
	return converted
}
