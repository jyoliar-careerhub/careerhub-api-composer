package userinfo

import (
	"context"

	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/common/domain"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/common/query"
	restapi_grpc "github.com/jyoliar-careerhub/careerhub-userinfo-service/api/restapi"
)

type MatchJobService interface {
	FindMatchJob(ctx context.Context, userId string) (*domain.MatchJob, error)
	AddCondition(ctx context.Context, userId string, limitCount uint32, req *AddConditionRequest) (*IsSuccessResponse, error)
	UpdateCondition(ctx context.Context, userId string, req *UpdateConditionRequest) (*IsSuccessResponse, error)
	DeleteCondition(ctx context.Context, userId string, conditionId string) (*IsSuccessResponse, error)
	UpdateAgreeToMail(ctx context.Context, userId string, agreeToMail bool) (*IsSuccessResponse, error)
}

type MatchJobServiceImpl struct {
	matchJobClient restapi_grpc.MatchJobGrpcClient
}

func NewMatchJobService(matchJobClient restapi_grpc.MatchJobGrpcClient) MatchJobService {
	return &MatchJobServiceImpl{
		matchJobClient: matchJobClient,
	}
}

func (s *MatchJobServiceImpl) FindMatchJob(ctx context.Context, userId string) (*domain.MatchJob, error) {
	res, err := s.matchJobClient.FindMatchJob(ctx, &restapi_grpc.FindMatchJobRequest{
		UserId: userId,
	})

	if err != nil {
		return nil, err
	}

	return &domain.MatchJob{
		AgreeToMail: res.AgreeToMail,
		Conditions:  convertToConditions(res),
	}, nil
}

func convertToConditions(res *restapi_grpc.FindMatchJobResponse) []*domain.Condition {
	conditions := make([]*domain.Condition, len(res.Conditions))
	for i, condition := range res.Conditions {
		skills := make([][]string, len(condition.Query.SkillNames))
		for j, skill := range condition.Query.SkillNames {
			skills[j] = skill.Or
		}

		categories := make([]*query.CategoryQuery, len(condition.Query.Categories))
		for j, category := range condition.Query.Categories {
			categories[j] = &query.CategoryQuery{
				Site:         category.Site,
				CategoryName: category.CategoryName,
			}
		}

		conditions[i] = &domain.Condition{
			ConditionId:   condition.ConditionId,
			ConditionName: condition.ConditionName,
			Query: &query.Query{
				MinCareer:  condition.Query.MinCareer,
				MaxCareer:  condition.Query.MaxCareer,
				SkillNames: skills,
				Categories: categories,
			},
		}
	}

	return conditions
}

func (s *MatchJobServiceImpl) AddCondition(ctx context.Context, userId string, limitCount uint32, req *AddConditionRequest) (*IsSuccessResponse, error) {

	res, err := s.matchJobClient.AddCondition(ctx, &restapi_grpc.AddConditionRequest{
		UserId:     userId,
		LimitCount: limitCount,
		Condition: &restapi_grpc.AddConditionReq{
			ConditionName: req.ConditionName,
			Query:         convertQueryToGrpc(req.Query),
		},
	})

	return convertGrpcToIsSuccessResponse(res), err
}

func (s *MatchJobServiceImpl) UpdateCondition(ctx context.Context, userId string, req *UpdateConditionRequest) (*IsSuccessResponse, error) {
	res, err := s.matchJobClient.UpdateCondition(ctx, &restapi_grpc.UpdateConditionRequest{
		UserId: userId,
		Condition: &restapi_grpc.Condition{
			ConditionId:   req.ConditionId,
			ConditionName: req.ConditionName,
			Query:         convertQueryToGrpc(req.Query),
		},
	})

	return convertGrpcToIsSuccessResponse(res), err
}

func convertQueryToGrpc(query *query.Query) *restapi_grpc.Query {
	categories := make([]*restapi_grpc.Category, len(query.Categories))
	for i, category := range query.Categories {
		categories[i] = &restapi_grpc.Category{
			Site:         category.Site,
			CategoryName: category.CategoryName,
		}
	}

	skillNames := make([]*restapi_grpc.Skill, len(query.SkillNames))
	for i, skillName := range query.SkillNames {
		skillNames[i] = &restapi_grpc.Skill{
			Or: skillName,
		}
	}

	return &restapi_grpc.Query{
		Categories: categories,
		SkillNames: skillNames,
		MinCareer:  query.MinCareer,
		MaxCareer:  query.MaxCareer,
	}

}

func (s *MatchJobServiceImpl) DeleteCondition(ctx context.Context, userId string, conditionId string) (*IsSuccessResponse, error) {
	res, err := s.matchJobClient.DeleteCondition(ctx, &restapi_grpc.DeleteConditionRequest{
		UserId:      userId,
		ConditionId: conditionId,
	})

	return convertGrpcToIsSuccessResponse(res), err
}

func (s *MatchJobServiceImpl) UpdateAgreeToMail(ctx context.Context, userId string, agreeToMail bool) (*IsSuccessResponse, error) {
	res, err := s.matchJobClient.UpdateAgreeToMail(ctx, &restapi_grpc.UpdateAgreeToMailRequest{
		UserId:      userId,
		AgreeToMail: agreeToMail,
	})

	return convertGrpcToIsSuccessResponse(res), err
}

func convertGrpcToIsSuccessResponse(res *restapi_grpc.IsSuccessResponse) *IsSuccessResponse {
	if res == nil {
		return nil
	}

	return &IsSuccessResponse{
		IsSuccess: res.IsSuccess,
	}
}
