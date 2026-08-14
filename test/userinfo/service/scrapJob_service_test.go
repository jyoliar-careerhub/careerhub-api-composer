package service

// import (
// 	"context"
// 	"fmt"
// 	"testing"

// 	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/dto"
// 	postingGrpc "github.com/jyoliar-careerhub/careerhub-posting-service/api/restapi"
// 	scrapJobGrpc "github.com/jyoliar-careerhub/careerhub-userinfo-service/api/restapi"
// 	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/userinfo/service"
// 	"github.com/jae2274/goutils/ptr"
// 	"github.com/stretchr/testify/require"
// 	"google.golang.org/grpc"
// 	"google.golang.org/protobuf/types/known/emptypb"
// )

// func TestScrapJobService(t *testing.T) {

// 	t.Run("return empty scrap job", func(t *testing.T) {
// 		scrapJobSvc := service.NewScrapJobService(InitMockClients([]*scrapJobGrpc.ScrapJob{}, []*postingGrpc.JobPostingRes{}))
// 		ctx := context.Background()
// 		res, err := scrapJobSvc.GetScrapJobs(ctx, "testUserId")
// 		require.NoError(t, err)
// 		require.Empty(t, res.JobPostings)
// 	})

// 	t.Run("return scrap job", func(t *testing.T) {
// 		posting1 := newJobPostingRes("site1", "posting1", 1)
// 		posting2 := newJobPostingRes("site2", "posting2", 2)
// 		posting3 := newJobPostingRes("site3", "posting3", 3)

// 		postings := []*postingGrpc.JobPostingRes{posting1, posting2, posting3}

// 		postingIds := []*scrapJobGrpc.ScrapJob{
// 			{Site: "site1", PostingId: "posting1"},
// 			{Site: "site2", PostingId: "posting2"},
// 			{Site: "site3", PostingId: "posting3"},
// 		}
// 		scrapJobSvc := service.NewScrapJobService(
// 			InitMockClients(postingIds, []*postingGrpc.JobPostingRes{posting1, posting3, posting2}),
// 		)
// 		ctx := context.Background()
// 		res, err := scrapJobSvc.GetScrapJobs(ctx, "testUserId")
// 		require.NoError(t, err)
// 		require.Len(t, res.JobPostings, 3)

// 		for i, jobPosting := range res.JobPostings {
// 			assertEaualJobPosting(t, postings[i], jobPosting)
// 		}
// 	})
// }

// func assertEaualJobPosting(t *testing.T, expected *postingGrpc.JobPostingRes, actual *dto.JobPostingRes) {
// 	require.Equal(t, expected.Site, actual.Site)
// 	require.Equal(t, expected.PostingId, actual.PostingId)
// 	require.Equal(t, expected.Title, actual.Title)
// 	require.Equal(t, expected.CompanyName, actual.CompanyName)
// 	require.Equal(t, expected.Skills, actual.Skills)
// 	require.Equal(t, expected.Categories, actual.Categories)
// 	require.Equal(t, expected.ImageUrl, actual.ImageUrl)
// 	require.Equal(t, expected.Addresses, actual.Addresses)
// 	require.Equal(t, expected.MinCareer, actual.MinCareer)
// 	require.Equal(t, expected.MaxCareer, actual.MaxCareer)
// 	require.True(t, actual.IsScrapped)
// }

// func InitMockClients(scrapJobIds []*scrapJobGrpc.ScrapJob, postings []*postingGrpc.JobPostingRes) (scrapJobGrpc.ScrapJobGrpcClient, postingGrpc.RestApiGrpcClient) {
// 	return &mockScrapJobGrpcClient{
// 			willReturn: &scrapJobGrpc.GetScrapJobsResponse{
// 				ScrapJobs: scrapJobIds,
// 			},
// 		}, &mockPostingGrpcClient{
// 			willReturn: &postingGrpc.JobPostingsResponse{
// 				JobPostings: postings,
// 			},
// 		}
// }

// type mockScrapJobGrpcClient struct {
// 	willReturn *scrapJobGrpc.GetScrapJobsResponse
// }

// func (m *mockScrapJobGrpcClient) GetScrapJobs(ctx context.Context, in *scrapJobGrpc.GetScrapJobsRequest, opts ...grpc.CallOption) (*scrapJobGrpc.GetScrapJobsResponse, error) {
// 	return m.willReturn, nil
// }

// var errNotImplemented = fmt.Errorf("not implemented")

// func (m *mockScrapJobGrpcClient) AddScrapJob(ctx context.Context, in *scrapJobGrpc.AddScrapJobRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
// 	return nil, errNotImplemented
// }
// func (m *mockScrapJobGrpcClient) RemoveScrapJob(ctx context.Context, in *scrapJobGrpc.RemoveScrapJobRequest, opts ...grpc.CallOption) (*scrapJobGrpc.IsExistedResponse, error) {
// 	return nil, errNotImplemented
// }
// func (m *mockScrapJobGrpcClient) AddTag(ctx context.Context, in *scrapJobGrpc.AddTagRequest, opts ...grpc.CallOption) (*scrapJobGrpc.IsExistedResponse, error) {
// 	return nil, errNotImplemented
// }
// func (m *mockScrapJobGrpcClient) RemoveTag(ctx context.Context, in *scrapJobGrpc.RemoveTagRequest, opts ...grpc.CallOption) (*scrapJobGrpc.IsExistedResponse, error) {
// 	return nil, errNotImplemented
// }
// func (m *mockScrapJobGrpcClient) GetScrapTags(ctx context.Context, in *scrapJobGrpc.GetScrapTagsRequest, opts ...grpc.CallOption) (*scrapJobGrpc.GetScrapTagsResponse, error) {
// 	return nil, errNotImplemented
// }

// type mockPostingGrpcClient struct {
// 	willReturn *postingGrpc.JobPostingsResponse
// }

// func (m *mockPostingGrpcClient) JobPostings(ctx context.Context, in *postingGrpc.JobPostingsRequest, opts ...grpc.CallOption) (*postingGrpc.JobPostingsResponse, error) {
// 	return m.willReturn, nil
// }
// func (m *mockPostingGrpcClient) JobPostingDetail(ctx context.Context, in *postingGrpc.JobPostingDetailRequest, opts ...grpc.CallOption) (*postingGrpc.JobPostingDetailResponse, error) {
// 	return nil, errNotImplemented
// }
// func (m *mockPostingGrpcClient) Categories(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*postingGrpc.CategoriesResponse, error) {
// 	return nil, errNotImplemented
// }
// func (m *mockPostingGrpcClient) Skills(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*postingGrpc.SkillsResponse, error) {
// 	return nil, errNotImplemented
// }

// func newJobPostingRes(site, postingId string, number int) *postingGrpc.JobPostingRes {
// 	attachN := func(s string, n int) string {
// 		return fmt.Sprintf("%s%d", s, n)
// 	}
// 	return &postingGrpc.JobPostingRes{
// 		Site:        site,
// 		PostingId:   postingId,
// 		Title:       attachN("title", number),
// 		CompanyName: attachN("company", number),
// 		Skills:      []string{attachN("skill", number)},
// 		Categories:  []string{attachN("category", number)},
// 		ImageUrl:    ptr.P(attachN("image", number)),
// 		Addresses:   []string{attachN("address", number)},
// 		MinCareer:   ptr.P(int32(3)),
// 		MaxCareer:   ptr.P(int32(5)),
// 	}
// }
