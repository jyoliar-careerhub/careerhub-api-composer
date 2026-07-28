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

// 시그니처 구속은 strict.go의 전체 인터페이스 단언(var _ api.StrictServerInterface)이 한다 —
// 일괄 전환(#18)으로 파일럿 시절의 메서드별 구속 선언을 대체했다.

func (c *ReviewStrictController) GetCompanyReviews(ctx context.Context, request api.GetCompanyReviewsRequestObject) (api.GetCompanyReviewsResponseObject, error) {
	params := request.Params

	// 형식 검증(음수·빈 companyName 등)은 명세 검증 미들웨어(NewSpecValidationMW)가 이미 했다 —
	// 여기엔 기본값 적용만 남는다(#14의 패턴 정련: 구현에는 명세로 표현 불가한 검증만 남긴다)
	page := review.InitPage
	if params.Page != nil {
		page = *params.Page
	}
	size := review.DefaultSize
	if params.Size != nil {
		size = *params.Size
	}

	reviews, err := c.reviewService.GetReviews(ctx, params.CompanyName, page, size)
	if err != nil {
		// 오류 원문은 로그로만 — 응답에는 노출하지 않는다 (기존 err.Error() 노출은 에러 계약 개정으로 폐기)
		llog.LogErr(ctx, err)
		return internalError500[api.GetCompanyReviews500JSONResponse](), nil
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
