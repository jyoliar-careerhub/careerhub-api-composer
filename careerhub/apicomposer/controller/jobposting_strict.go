package controller

import (
	"context"
	"encoding/json"

	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/common/domain"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/common/query"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/middleware"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/posting"
	postingGrpc "github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/posting/restapi_grpc"
	"github.com/jae2274/goutils/llog"
)

// JobPostingStrictController — 전환 패턴은 review_strict.go 참고.
// 경로 파라미터·쿼리 파라미터의 파싱(base64 디코딩 포함)과 min/max 검증은 생성 코드·검증
// 미들웨어가 한다 — 여기엔 명세로 표현 불가한 검증(encoded_query의 JSON 파싱, minCareer ≤
// maxCareer 교차 검증, keyword 비어있음)만 남는다. 공개 그룹: 비로그인 접근 가능,
// 로그인 시 scrapInfo가 부가된다(claims 유무로 분기 — 기존과 동일).
type JobPostingStrictController struct {
	postingService *posting.PostingService
}

func NewJobPostingStrictController(postingService *posting.PostingService) *JobPostingStrictController {
	return &JobPostingStrictController{postingService: postingService}
}

const cacheControlDay = "max-age=86400, public"
const cacheControlHalfDay = "max-age=43200, public"

// parseEncodedQuery — base64 디코딩은 생성 바인딩이 이미 끝냈다. JSON 파싱·검증만 한다.
// 오류 메시지는 기존 IsInvalidQuery의 wire("bad request" / IsValid의 검증 메시지)를 유지한다.
func parseEncodedQuery(ctx context.Context, decoded []byte) (*query.Query, *string) {
	var queryReq query.Query
	if err := json.Unmarshal(decoded, &queryReq); err != nil {
		llog.LogErr(ctx, err)
		msg := "bad request"
		return nil, &msg
	}
	if err := posting.IsValid(&queryReq); err != nil {
		msg := err.Error()
		return nil, &msg
	}
	return &queryReq, nil
}

func (c *JobPostingStrictController) GetJobPostings(ctx context.Context, request api.GetJobPostingsRequestObject) (api.GetJobPostingsResponseObject, error) {
	queryReq, errMsg := parseEncodedQuery(ctx, request.Params.EncodedQuery)
	if errMsg != nil {
		return api.GetJobPostings400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{Message: *errMsg}}, nil
	}

	req := &posting.JobPostingsRequest{
		Page:     int32(request.Params.Page),
		Size:     int32(request.Params.Size),
		QueryReq: queryReq,
	}

	var jobPostings []*domain.JobPosting
	var err error
	if claims, isExisted := middleware.GetClaims(ctx); isExisted {
		jobPostings, err = c.postingService.JobPostingsWithClaims(ctx, req, claims)
	} else {
		jobPostings, err = c.postingService.JobPostings(ctx, req)
	}
	if err != nil {
		llog.LogErr(ctx, err)
		return api.GetJobPostings500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Message: messageInternalServerError}}, nil
	}

	return api.GetJobPostings200JSONResponse(convertJobPostingsToAPI(jobPostings)), nil
}

func (c *JobPostingStrictController) CountJobPostings(ctx context.Context, request api.CountJobPostingsRequestObject) (api.CountJobPostingsResponseObject, error) {
	queryReq, errMsg := parseEncodedQuery(ctx, request.Params.EncodedQuery)
	if errMsg != nil {
		return api.CountJobPostings400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{Message: *errMsg}}, nil
	}

	count, err := c.postingService.CountJobPostings(ctx, queryReq)
	if err != nil {
		llog.LogErr(ctx, err)
		return api.CountJobPostings500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Message: messageInternalServerError}}, nil
	}

	return api.CountJobPostings200JSONResponse(api.CountResponse{Count: count}), nil
}

func (c *JobPostingStrictController) GetJobPostingDetail(ctx context.Context, request api.GetJobPostingDetailRequestObject) (api.GetJobPostingDetailResponseObject, error) {
	req := &postingGrpc.JobPostingDetailRequest{
		Site:      request.Site,
		PostingId: request.PostingId,
	}

	var resp *domain.JobPostingDetail
	var isExist bool
	var err error
	if claims, isExisted := middleware.GetClaims(ctx); isExisted {
		resp, isExist, err = c.postingService.JobPostingDetailWithClaims(ctx, req, claims)
	} else {
		resp, isExist, err = c.postingService.JobPostingDetail(ctx, req)
	}
	if err != nil {
		llog.LogErr(ctx, err)
		return api.GetJobPostingDetail500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Message: messageInternalServerError}}, nil
	}
	if !isExist {
		return api.GetJobPostingDetail404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: "job posting not found"}}, nil
	}

	return api.GetJobPostingDetail200JSONResponse(convertJobPostingDetailToAPI(resp)), nil
}

func (c *JobPostingStrictController) GetCategories(ctx context.Context, request api.GetCategoriesRequestObject) (api.GetCategoriesResponseObject, error) {
	categories, err := c.postingService.Categories(ctx)
	if err != nil {
		llog.LogErr(ctx, err)
		return api.GetCategories500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Message: messageInternalServerError}}, nil
	}

	var bySite *[]api.CategoryRes
	if len(categories.CategoriesBySite) > 0 {
		converted := make([]api.CategoryRes, len(categories.CategoriesBySite))
		for i, c := range categories.CategoriesBySite {
			converted[i] = api.CategoryRes{
				Site:       strOrOmit(c.Site),
				Categories: sliceOrOmit(c.Categories),
			}
		}
		bySite = &converted
	}

	cacheControl := cacheControlDay
	return api.GetCategories200JSONResponse{
		Body:    api.CategoriesResponse{CategoriesBySite: bySite},
		Headers: api.GetCategories200ResponseHeaders{CacheControl: &cacheControl},
	}, nil
}

func (c *JobPostingStrictController) GetSkills(ctx context.Context, request api.GetSkillsRequestObject) (api.GetSkillsResponseObject, error) {
	skills, err := c.postingService.Skills(ctx)
	if err != nil {
		llog.LogErr(ctx, err)
		return api.GetSkills500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Message: messageInternalServerError}}, nil
	}

	var converted *[]api.SkillRes
	if len(skills.Skills) > 0 {
		items := make([]api.SkillRes, len(skills.Skills))
		for i, s := range skills.Skills {
			items[i] = api.SkillRes{
				DefaultName: strOrOmit(s.DefaultName),
				SkillNames:  sliceOrOmit(s.SkillNames),
			}
		}
		converted = &items
	}

	cacheControl := cacheControlDay
	return api.GetSkills200JSONResponse{
		Body:    api.SkillsResponse{Skills: converted},
		Headers: api.GetSkills200ResponseHeaders{CacheControl: &cacheControl},
	}, nil
}

func (c *JobPostingStrictController) GetCompanies(ctx context.Context, request api.GetCompaniesRequestObject) (api.GetCompaniesResponseObject, error) {
	// keyword 비어있음(minLength)은 명세 검증 미들웨어가 거부한다
	companies, err := c.postingService.Companies(ctx, request.Params.Keyword)
	if err != nil {
		llog.LogErr(ctx, err)
		return api.GetCompanies500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Message: messageInternalServerError}}, nil
	}

	var converted *[]api.CompanyRes
	if len(companies.Companies) > 0 {
		items := make([]api.CompanyRes, len(companies.Companies))
		for i, comp := range companies.Companies {
			var siteCompanies *[]api.SiteCompanyRes
			if len(comp.SiteCompanies) > 0 {
				scs := make([]api.SiteCompanyRes, len(comp.SiteCompanies))
				for j, sc := range comp.SiteCompanies {
					scs[j] = api.SiteCompanyRes{
						Site:        strOrOmit(sc.Site),
						CompanyId:   strOrOmit(sc.CompanyId),
						CompanyName: strOrOmit(sc.CompanyName),
					}
				}
				siteCompanies = &scs
			}
			items[i] = api.CompanyRes{
				DefaultName:   strOrOmit(comp.DefaultName),
				SiteCompanies: siteCompanies,
			}
		}
		converted = &items
	}

	cacheControl := cacheControlHalfDay
	return api.GetCompanies200JSONResponse{
		Body:    api.CompaniesResponse{Companies: converted},
		Headers: api.GetCompanies200ResponseHeaders{CacheControl: &cacheControl},
	}, nil
}
