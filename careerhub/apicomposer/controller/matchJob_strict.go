package controller

import (
	"context"

	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/common/domain"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/common/query"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/middleware"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/userinfo"
	"github.com/jae2274/goutils/llog"
)

// MatchJobStrictController — 전환 패턴은 review_strict.go 참고.
// 로그인 검사는 명세 security 기반 AuthenticationFunc가, 권한(AUTHORITY_MATCH_JOB) 검사는
// 배선의 권한 미들웨어(strict.go의 requireAuthorityMW)가 한다 — 여기 도달 시 claims는
// 항상 존재하고, 핸들러는 UserId 사용을 위해 조회만 한다.
type MatchJobStrictController struct {
	matchJobSvc userinfo.MatchJobService
}

func NewMatchJobStrictController(matchJobSvc userinfo.MatchJobService) *MatchJobStrictController {
	return &MatchJobStrictController{matchJobSvc: matchJobSvc}
}

// 사용자당 매칭 조건 상한 — 기존 손 핸들러(matchJob_controller.go)의 값 유지
const conditionLimitCount = 2

func (c *MatchJobStrictController) FindMatchJob(ctx context.Context, request api.FindMatchJobRequestObject) (api.FindMatchJobResponseObject, error) {
	claims, _ := middleware.GetClaims(ctx)

	matchJob, err := c.matchJobSvc.FindMatchJob(ctx, claims.UserId)
	if err != nil {
		llog.LogErr(ctx, err)
		return internalError500[api.FindMatchJob500JSONResponse](), nil
	}

	return api.FindMatchJob200JSONResponse(convertMatchJobToAPI(matchJob)), nil
}

func convertMatchJobToAPI(matchJob *domain.MatchJob) api.MatchJob {
	conditions := make([]api.Condition, len(matchJob.Conditions))
	for i, cond := range matchJob.Conditions {
		conditions[i] = api.Condition{
			ConditionId:   cond.ConditionId,
			ConditionName: cond.ConditionName,
			Query:         convertQueryToAPI(cond.Query),
		}
	}
	return api.MatchJob{
		Conditions:  conditions,
		AgreeToMail: matchJob.AgreeToMail,
	}
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

func (c *MatchJobStrictController) AddMatchJobCondition(ctx context.Context, request api.AddMatchJobConditionRequestObject) (api.AddMatchJobConditionResponseObject, error) {
	claims, _ := middleware.GetClaims(ctx)

	res, err := c.matchJobSvc.AddCondition(ctx, claims.UserId, conditionLimitCount, &userinfo.AddConditionRequest{
		ConditionName: request.Body.ConditionName,
		Query:         convertAPIQueryToDomain(request.Body.Query),
	})
	if err != nil {
		llog.LogErr(ctx, err)
		return internalError500[api.AddMatchJobCondition500JSONResponse](), nil
	}

	return api.AddMatchJobCondition200JSONResponse(api.IsSuccessResponse{IsSuccess: res.IsSuccess}), nil
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

func (c *MatchJobStrictController) UpdateMatchJobCondition(ctx context.Context, request api.UpdateMatchJobConditionRequestObject) (api.UpdateMatchJobConditionResponseObject, error) {
	claims, _ := middleware.GetClaims(ctx)

	res, err := c.matchJobSvc.UpdateCondition(ctx, claims.UserId, &userinfo.UpdateConditionRequest{
		ConditionId:   request.Body.ConditionId,
		ConditionName: request.Body.ConditionName,
		Query:         convertAPIQueryToDomain(request.Body.Query),
	})
	if err != nil {
		llog.LogErr(ctx, err)
		return internalError500[api.UpdateMatchJobCondition500JSONResponse](), nil
	}

	return api.UpdateMatchJobCondition200JSONResponse(api.IsSuccessResponse{IsSuccess: res.IsSuccess}), nil
}

func (c *MatchJobStrictController) DeleteMatchJobCondition(ctx context.Context, request api.DeleteMatchJobConditionRequestObject) (api.DeleteMatchJobConditionResponseObject, error) {
	claims, _ := middleware.GetClaims(ctx)

	res, err := c.matchJobSvc.DeleteCondition(ctx, claims.UserId, request.Body.ConditionId)
	if err != nil {
		llog.LogErr(ctx, err)
		return internalError500[api.DeleteMatchJobCondition500JSONResponse](), nil
	}

	return api.DeleteMatchJobCondition200JSONResponse(api.IsSuccessResponse{IsSuccess: res.IsSuccess}), nil
}

func (c *MatchJobStrictController) UpdateAgreeToMail(ctx context.Context, request api.UpdateAgreeToMailRequestObject) (api.UpdateAgreeToMailResponseObject, error) {
	claims, _ := middleware.GetClaims(ctx)

	res, err := c.matchJobSvc.UpdateAgreeToMail(ctx, claims.UserId, request.Body.AgreeToMail)
	if err != nil {
		llog.LogErr(ctx, err)
		return internalError500[api.UpdateAgreeToMail500JSONResponse](), nil
	}

	return api.UpdateAgreeToMail200JSONResponse(api.IsSuccessResponse{IsSuccess: res.IsSuccess}), nil
}
