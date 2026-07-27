package controller

import (
	"context"

	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/middleware"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/userinfo"
	scrapGrpc "github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/userinfo/restapi_grpc"
	"github.com/jae2274/goutils/llog"
	"github.com/jae2274/goutils/terr"
)

// ScrapJobStrictController — 전환 패턴은 review_strict.go 참고.
// 요청 본문이 있는 오퍼레이션(POST·DELETE)의 첫 적용: 본문 파싱은 생성 코드가 하고,
// 디코드 실패는 배선의 RequestErrorHandlerFunc가 ErrorResponse 400으로 응답한다
// (기존 손 핸들러의 500은 에러 계약 개정으로 폐기).
// 요청 본문의 userId 필드는 명세에 없다 — 항상 토큰의 userId를 쓴다(기존 덮어쓰기와 동일 결과).
type ScrapJobStrictController struct {
	scrapJobSvc *userinfo.ScrapJobService
}

func NewScrapJobStrictController(scrapJobSvc *userinfo.ScrapJobService) *ScrapJobStrictController {
	return &ScrapJobStrictController{scrapJobSvc: scrapJobSvc}
}

const messageUnauthorized = "Unauthorized"
const messageScrapJobNotFound = "Scrap job not found"

func (c *ScrapJobStrictController) GetScrapJobs(ctx context.Context, request api.GetScrapJobsRequestObject) (api.GetScrapJobsResponseObject, error) {
	claims, ok := middleware.GetClaims(ctx)
	if !ok {
		llog.LogErr(ctx, terr.New("No claims in context"))
		return api.GetScrapJobs401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse{Message: messageUnauthorized}}, nil
	}

	jobPostings, err := c.scrapJobSvc.GetScrapJobs(ctx, claims, request.Params.Tag)
	if err != nil {
		llog.LogErr(ctx, err)
		return api.GetScrapJobs500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Message: messageInternalServerError}}, nil
	}

	return api.GetScrapJobs200JSONResponse(convertJobPostingsToAPI(jobPostings)), nil
}

func (c *ScrapJobStrictController) AddScrapJob(ctx context.Context, request api.AddScrapJobRequestObject) (api.AddScrapJobResponseObject, error) {
	claims, ok := middleware.GetClaims(ctx)
	if !ok {
		llog.LogErr(ctx, terr.New("No claims in context"))
		return api.AddScrapJob401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse{Message: messageUnauthorized}}, nil
	}

	err := c.scrapJobSvc.AddScrapJob(ctx, &scrapGrpc.AddScrapJobRequest{
		UserId:    claims.UserId,
		Site:      request.Body.Site,
		PostingId: request.Body.PostingId,
	})
	if err != nil {
		llog.LogErr(ctx, err)
		return api.AddScrapJob500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Message: messageInternalServerError}}, nil
	}

	return api.AddScrapJob201Response{}, nil
}

func (c *ScrapJobStrictController) RemoveScrapJob(ctx context.Context, request api.RemoveScrapJobRequestObject) (api.RemoveScrapJobResponseObject, error) {
	claims, ok := middleware.GetClaims(ctx)
	if !ok {
		llog.LogErr(ctx, terr.New("No claims in context"))
		return api.RemoveScrapJob401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse{Message: messageUnauthorized}}, nil
	}

	isExisted, err := c.scrapJobSvc.RemoveScrapJob(ctx, &scrapGrpc.RemoveScrapJobRequest{
		UserId:    claims.UserId,
		Site:      request.Body.Site,
		PostingId: request.Body.PostingId,
	})
	if err != nil {
		llog.LogErr(ctx, err)
		return api.RemoveScrapJob500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Message: messageInternalServerError}}, nil
	}
	if !isExisted {
		return api.RemoveScrapJob404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: messageScrapJobNotFound}}, nil
	}

	return api.RemoveScrapJob200Response{}, nil
}

func (c *ScrapJobStrictController) GetUntaggedScrapJobs(ctx context.Context, request api.GetUntaggedScrapJobsRequestObject) (api.GetUntaggedScrapJobsResponseObject, error) {
	claims, ok := middleware.GetClaims(ctx)
	if !ok {
		llog.LogErr(ctx, terr.New("No claims in context"))
		return api.GetUntaggedScrapJobs401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse{Message: messageUnauthorized}}, nil
	}

	scrapJobs, err := c.scrapJobSvc.GetUntaggedScrapJobs(ctx, claims.UserId)
	if err != nil {
		llog.LogErr(ctx, err)
		return api.GetUntaggedScrapJobs500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Message: messageInternalServerError}}, nil
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
	claims, ok := middleware.GetClaims(ctx)
	if !ok {
		llog.LogErr(ctx, terr.New("No claims in context"))
		return api.GetScrapTags401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse{Message: messageUnauthorized}}, nil
	}

	res, err := c.scrapJobSvc.GetScrapTags(ctx, claims.UserId)
	if err != nil {
		llog.LogErr(ctx, err)
		return api.GetScrapTags500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Message: messageInternalServerError}}, nil
	}

	return api.GetScrapTags200JSONResponse(api.ScrapTagsResponse{Tags: sliceOrOmit(res.Tags)}), nil
}

func (c *ScrapJobStrictController) AddScrapTag(ctx context.Context, request api.AddScrapTagRequestObject) (api.AddScrapTagResponseObject, error) {
	claims, ok := middleware.GetClaims(ctx)
	if !ok {
		llog.LogErr(ctx, terr.New("No claims in context"))
		return api.AddScrapTag401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse{Message: messageUnauthorized}}, nil
	}

	isExisted, err := c.scrapJobSvc.AddTag(ctx, &scrapGrpc.AddTagRequest{
		UserId:    claims.UserId,
		Site:      request.Body.Site,
		PostingId: request.Body.PostingId,
		Tag:       request.Body.Tag,
	})
	if err != nil {
		llog.LogErr(ctx, err)
		return api.AddScrapTag500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Message: messageInternalServerError}}, nil
	}
	if !isExisted {
		return api.AddScrapTag404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: messageScrapJobNotFound}}, nil
	}

	return api.AddScrapTag200Response{}, nil
}

func (c *ScrapJobStrictController) RemoveScrapTag(ctx context.Context, request api.RemoveScrapTagRequestObject) (api.RemoveScrapTagResponseObject, error) {
	claims, ok := middleware.GetClaims(ctx)
	if !ok {
		llog.LogErr(ctx, terr.New("No claims in context"))
		return api.RemoveScrapTag401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse{Message: messageUnauthorized}}, nil
	}

	isExisted, err := c.scrapJobSvc.RemoveTag(ctx, &scrapGrpc.RemoveTagRequest{
		UserId:    claims.UserId,
		Site:      request.Body.Site,
		PostingId: request.Body.PostingId,
		Tag:       request.Body.Tag,
	})
	if err != nil {
		llog.LogErr(ctx, err)
		return api.RemoveScrapTag500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Message: messageInternalServerError}}, nil
	}
	if !isExisted {
		return api.RemoveScrapTag404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: messageScrapJobNotFound}}, nil
	}

	return api.RemoveScrapTag200Response{}, nil
}
