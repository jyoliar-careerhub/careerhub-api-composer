package controller

import (
	"context"

	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/middleware"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/userinfo"
	scrapGrpc "github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/userinfo/restapi_grpc"
	"github.com/jae2274/goutils/llog"
)

// ScrapJobStrictController — 전환 패턴은 review_strict.go 참고.
// 요청 본문이 있는 오퍼레이션(POST·DELETE)의 첫 적용: 본문 파싱은 생성 코드가 하고,
// 디코드 실패는 배선의 RequestErrorHandlerFunc가 ErrorResponse 400으로 응답한다
// (기존 손 핸들러의 500은 에러 계약 개정으로 폐기).
// 요청 본문의 userId 필드는 명세에 없다 — 항상 토큰의 userId를 쓴다(기존 덮어쓰기와 동일 결과).
// 로그인 검사는 명세 security 기반 AuthenticationFunc가 한다 — 여기 도달 시 claims는 항상
// 존재하고, 핸들러는 UserId 사용을 위해 조회만 한다.
type ScrapJobStrictController struct {
	scrapJobSvc *userinfo.ScrapJobService
}

func NewScrapJobStrictController(scrapJobSvc *userinfo.ScrapJobService) *ScrapJobStrictController {
	return &ScrapJobStrictController{scrapJobSvc: scrapJobSvc}
}

const messageScrapJobNotFound = "Scrap job not found"

func (c *ScrapJobStrictController) GetScrapJobs(ctx context.Context, request api.GetScrapJobsRequestObject) (api.GetScrapJobsResponseObject, error) {
	claims, _ := middleware.GetClaims(ctx)

	jobPostings, err := c.scrapJobSvc.GetScrapJobs(ctx, claims, request.Params.Tag)
	if err != nil {
		llog.LogErr(ctx, err)
		return internalError500[api.GetScrapJobs500JSONResponse](), nil
	}

	return api.GetScrapJobs200JSONResponse(convertJobPostingsToAPI(jobPostings)), nil
}

func (c *ScrapJobStrictController) AddScrapJob(ctx context.Context, request api.AddScrapJobRequestObject) (api.AddScrapJobResponseObject, error) {
	claims, _ := middleware.GetClaims(ctx)

	err := c.scrapJobSvc.AddScrapJob(ctx, &scrapGrpc.AddScrapJobRequest{
		UserId:    claims.UserId,
		Site:      request.Body.Site,
		PostingId: request.Body.PostingId,
	})
	if err != nil {
		llog.LogErr(ctx, err)
		return internalError500[api.AddScrapJob500JSONResponse](), nil
	}

	return api.AddScrapJob201Response{}, nil
}

func (c *ScrapJobStrictController) RemoveScrapJob(ctx context.Context, request api.RemoveScrapJobRequestObject) (api.RemoveScrapJobResponseObject, error) {
	claims, _ := middleware.GetClaims(ctx)

	isExisted, err := c.scrapJobSvc.RemoveScrapJob(ctx, &scrapGrpc.RemoveScrapJobRequest{
		UserId:    claims.UserId,
		Site:      request.Body.Site,
		PostingId: request.Body.PostingId,
	})
	if err != nil {
		llog.LogErr(ctx, err)
		return internalError500[api.RemoveScrapJob500JSONResponse](), nil
	}
	if !isExisted {
		return notFound404[api.RemoveScrapJob404JSONResponse](messageScrapJobNotFound), nil
	}

	return api.RemoveScrapJob200Response{}, nil
}

func (c *ScrapJobStrictController) GetUntaggedScrapJobs(ctx context.Context, request api.GetUntaggedScrapJobsRequestObject) (api.GetUntaggedScrapJobsResponseObject, error) {
	claims, _ := middleware.GetClaims(ctx)

	scrapJobs, err := c.scrapJobSvc.GetUntaggedScrapJobs(ctx, claims.UserId)
	if err != nil {
		llog.LogErr(ctx, err)
		return internalError500[api.GetUntaggedScrapJobs500JSONResponse](), nil
	}

	return api.GetUntaggedScrapJobs200JSONResponse(convertScrapJobsToAPI(scrapJobs)), nil
}

// convertScrapJobsToAPI — 기존 wire 유지: nil 슬라이스는 JSON null로, 항목의 빈 필드는 생략(proto omitempty).
func convertScrapJobsToAPI(scrapJobs []*scrapGrpc.ScrapJob) []api.ScrapJob {
	if scrapJobs == nil {
		return nil
	}
	converted := make([]api.ScrapJob, len(scrapJobs))
	for i, sj := range scrapJobs {
		converted[i] = api.ScrapJob{
			Site:      strOrOmit(sj.Site),
			PostingId: strOrOmit(sj.PostingId),
			Tags:      sliceOrOmit(sj.Tags),
		}
	}
	return converted
}

func (c *ScrapJobStrictController) GetScrapTags(ctx context.Context, request api.GetScrapTagsRequestObject) (api.GetScrapTagsResponseObject, error) {
	claims, _ := middleware.GetClaims(ctx)

	res, err := c.scrapJobSvc.GetScrapTags(ctx, claims.UserId)
	if err != nil {
		llog.LogErr(ctx, err)
		return internalError500[api.GetScrapTags500JSONResponse](), nil
	}

	return api.GetScrapTags200JSONResponse(api.ScrapTagsResponse{Tags: sliceOrOmit(res.Tags)}), nil
}

func (c *ScrapJobStrictController) AddScrapTag(ctx context.Context, request api.AddScrapTagRequestObject) (api.AddScrapTagResponseObject, error) {
	claims, _ := middleware.GetClaims(ctx)

	isExisted, err := c.scrapJobSvc.AddTag(ctx, &scrapGrpc.AddTagRequest{
		UserId:    claims.UserId,
		Site:      request.Body.Site,
		PostingId: request.Body.PostingId,
		Tag:       request.Body.Tag,
	})
	if err != nil {
		llog.LogErr(ctx, err)
		return internalError500[api.AddScrapTag500JSONResponse](), nil
	}
	if !isExisted {
		return notFound404[api.AddScrapTag404JSONResponse](messageScrapJobNotFound), nil
	}

	return api.AddScrapTag200Response{}, nil
}

func (c *ScrapJobStrictController) RemoveScrapTag(ctx context.Context, request api.RemoveScrapTagRequestObject) (api.RemoveScrapTagResponseObject, error) {
	claims, _ := middleware.GetClaims(ctx)

	isExisted, err := c.scrapJobSvc.RemoveTag(ctx, &scrapGrpc.RemoveTagRequest{
		UserId:    claims.UserId,
		Site:      request.Body.Site,
		PostingId: request.Body.PostingId,
		Tag:       request.Body.Tag,
	})
	if err != nil {
		llog.LogErr(ctx, err)
		return internalError500[api.RemoveScrapTag500JSONResponse](), nil
	}
	if !isExisted {
		return notFound404[api.RemoveScrapTag404JSONResponse](messageScrapJobNotFound), nil
	}

	return api.RemoveScrapTag200Response{}, nil
}
