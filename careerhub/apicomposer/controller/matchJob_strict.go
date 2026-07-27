package controller

import (
	"context"

	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/common/query"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/middleware"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/userinfo"
	"github.com/jae2274/goutils/llog"
	"github.com/jae2274/goutils/terr"
)

// MatchJobStrictController — 전환 패턴은 review_strict.go 참고.
// 권한(AUTHORITY_MATCH_JOB) 검사는 여기가 아니라 배선의 권한 미들웨어가 한다
// (strict.go의 authorityMiddleware — 기존 CheckHasAuthority의 라우터 단위 검사를
// operationID 단위로 옮긴 것). 여기의 claims 부재 401은 방어적 검사다(기존과 동일).
type MatchJobStrictController struct {
	matchJobSvc userinfo.MatchJobService
}

func NewMatchJobStrictController(matchJobSvc userinfo.MatchJobService) *MatchJobStrictController {
	return &MatchJobStrictController{matchJobSvc: matchJobSvc}
}

// 사용자당 매칭 조건 상한 — 기존 손 핸들러(matchJob_controller.go)의 값 유지
const conditionLimitCount = 2

func (c *MatchJobStrictController) FindMatchJob(ctx context.Context, request api.FindMatchJobRequestObject) (api.FindMatchJobResponseObject, error) {
	claims, ok := middleware.GetClaims(ctx)
	if !ok {
		llog.LogErr(ctx, terr.New("No claims in context"))
		return api.FindMatchJob401Response{}, nil
	}

	matchJob, err := c.matchJobSvc.FindMatchJob(ctx, claims.UserId)
	if err != nil {
		llog.LogErr(ctx, err)
		return api.FindMatchJob500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Message: messageInternalServerError}}, nil
	}

	conditions := make([]api.Condition, len(matchJob.Conditions))
	for i, cond := range matchJob.Conditions {
		conditions[i] = api.Condition{
			ConditionId:   cond.ConditionId,
			ConditionName: cond.ConditionName,
			Query:         convertQueryToAPI(cond.Query),
		}
	}

	return api.FindMatchJob200JSONResponse(api.MatchJob{
		Conditions:  conditions,
		AgreeToMail: matchJob.AgreeToMail,
	}), nil
}

func (c *MatchJobStrictController) AddMatchJobCondition(ctx context.Context, request api.AddMatchJobConditionRequestObject) (api.AddMatchJobConditionResponseObject, error) {
	claims, ok := middleware.GetClaims(ctx)
	if !ok {
		llog.LogErr(ctx, terr.New("No claims in context"))
		return api.AddMatchJobCondition401Response{}, nil
	}

	res, err := c.matchJobSvc.AddCondition(ctx, claims.UserId, conditionLimitCount, &userinfo.AddConditionRequest{
		ConditionName: request.Body.ConditionName,
		Query:         convertAPIQueryToDomain(request.Body.Query),
	})
	if err != nil {
		llog.LogErr(ctx, err)
		return api.AddMatchJobCondition500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Message: messageInternalServerError}}, nil
	}

	return api.AddMatchJobCondition200JSONResponse(api.IsSuccessResponse{IsSuccess: res.IsSuccess}), nil
}

func (c *MatchJobStrictController) UpdateMatchJobCondition(ctx context.Context, request api.UpdateMatchJobConditionRequestObject) (api.UpdateMatchJobConditionResponseObject, error) {
	claims, ok := middleware.GetClaims(ctx)
	if !ok {
		llog.LogErr(ctx, terr.New("No claims in context"))
		return api.UpdateMatchJobCondition401Response{}, nil
	}

	res, err := c.matchJobSvc.UpdateCondition(ctx, claims.UserId, &userinfo.UpdateConditionRequest{
		ConditionId:   request.Body.ConditionId,
		ConditionName: request.Body.ConditionName,
		Query:         convertAPIQueryToDomain(request.Body.Query),
	})
	if err != nil {
		llog.LogErr(ctx, err)
		return api.UpdateMatchJobCondition500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Message: messageInternalServerError}}, nil
	}

	return api.UpdateMatchJobCondition200JSONResponse(api.IsSuccessResponse{IsSuccess: res.IsSuccess}), nil
}

func (c *MatchJobStrictController) DeleteMatchJobCondition(ctx context.Context, request api.DeleteMatchJobConditionRequestObject) (api.DeleteMatchJobConditionResponseObject, error) {
	claims, ok := middleware.GetClaims(ctx)
	if !ok {
		llog.LogErr(ctx, terr.New("No claims in context"))
		return api.DeleteMatchJobCondition401Response{}, nil
	}

	res, err := c.matchJobSvc.DeleteCondition(ctx, claims.UserId, request.Body.ConditionId)
	if err != nil {
		llog.LogErr(ctx, err)
		return api.DeleteMatchJobCondition500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Message: messageInternalServerError}}, nil
	}

	return api.DeleteMatchJobCondition200JSONResponse(api.IsSuccessResponse{IsSuccess: res.IsSuccess}), nil
}

func (c *MatchJobStrictController) UpdateAgreeToMail(ctx context.Context, request api.UpdateAgreeToMailRequestObject) (api.UpdateAgreeToMailResponseObject, error) {
	claims, ok := middleware.GetClaims(ctx)
	if !ok {
		llog.LogErr(ctx, terr.New("No claims in context"))
		return api.UpdateAgreeToMail401Response{}, nil
	}

	res, err := c.matchJobSvc.UpdateAgreeToMail(ctx, claims.UserId, request.Body.AgreeToMail)
	if err != nil {
		llog.LogErr(ctx, err)
		return api.UpdateAgreeToMail500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Message: messageInternalServerError}}, nil
	}

	return api.UpdateAgreeToMail200JSONResponse(api.IsSuccessResponse{IsSuccess: res.IsSuccess}), nil
}

// convertQueryToAPI — 응답 방향. categories·skillNames는 서비스가 항상 non-nil로 주므로
// 빈 배열도 []로 나간다(기존과 동일). minCareer·maxCareer·companies는 값이 없으면 필드가
// 생략된다 — 기존 wire는 null이었으나 명세(옵셔널 필드)에 맞춰 생략으로 수렴 (티켓 코멘트 기록).
func convertQueryToAPI(q *query.Query) api.Query {
	if q == nil {
		return api.Query{}
	}

	var categories *[]api.CategoryQuery
	if q.Categories != nil {
		converted := make([]api.CategoryQuery, len(q.Categories))
		for i, c := range q.Categories {
			converted[i] = api.CategoryQuery{Site: c.Site, CategoryName: c.CategoryName}
		}
		categories = &converted
	}

	var companies *[]api.SiteCompanyQuery
	if q.Companies != nil {
		converted := make([]api.SiteCompanyQuery, len(q.Companies))
		for i, c := range q.Companies {
			converted[i] = api.SiteCompanyQuery{Site: c.Site, CompanyName: c.CompanyName}
		}
		companies = &converted
	}

	return api.Query{
		Categories: categories,
		SkillNames: sliceOrNil(q.SkillNames),
		MinCareer:  q.MinCareer,
		MaxCareer:  q.MaxCareer,
		Companies:  companies,
	}
}

// convertAPIQueryToDomain — 요청 방향. 생략된 필드는 nil로 두고 서비스 계층의 기존
// nil 허용 동작에 맡긴다(기존 json 디코드 결과와 동일).
func convertAPIQueryToDomain(q api.Query) *query.Query {
	var categories []*query.CategoryQuery
	if q.Categories != nil {
		categories = make([]*query.CategoryQuery, len(*q.Categories))
		for i, c := range *q.Categories {
			categories[i] = &query.CategoryQuery{Site: c.Site, CategoryName: c.CategoryName}
		}
	}

	var companies []query.SiteCompanyQuery
	if q.Companies != nil {
		companies = make([]query.SiteCompanyQuery, len(*q.Companies))
		for i, c := range *q.Companies {
			companies[i] = query.SiteCompanyQuery{Site: c.Site, CompanyName: c.CompanyName}
		}
	}

	var skillNames [][]string
	if q.SkillNames != nil {
		skillNames = *q.SkillNames
	}

	return &query.Query{
		Categories: categories,
		SkillNames: skillNames,
		MinCareer:  q.MinCareer,
		MaxCareer:  q.MaxCareer,
		Companies:  companies,
	}
}
