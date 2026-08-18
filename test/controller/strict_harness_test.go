package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/controller"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/jwtresolver"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/middleware"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/posting"
	postingGrpc "github.com/jyoliar-careerhub/careerhub-posting-service/api/restapi"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/review"
	reviewGrpc "github.com/jyoliar-careerhub/careerhub-review-service/api/restapi"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/userinfo"
	scrapGrpc "github.com/jyoliar-careerhub/careerhub-userinfo-service/api/restapi"
	"github.com/jae2274/careerhub-api-composer/test/tokentest"
	"github.com/jae2274/goutils/mw"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

const testPrefix = "/api" // ROOT_PATH prefix — 생성 배선·검증 미들웨어가 prefix 아래에서도 동작함을 함께 실증

// testZitadel — 패키지 공용 가짜 발급자(FakeZitadel: RSA 서명 키 + JWKS 엔드포인트).
// 서버별 resolver는 newStrictTestServer가 만들고, 토큰 발급은 newToken이 여기서 받는다.
var testZitadel *tokentest.FakeZitadel

func TestMain(m *testing.M) {
	testZitadel = tokentest.StartFakeZitadel()
	code := m.Run()
	testZitadel.Close()
	os.Exit(code)
}

// ctxCapture는 업스트림 호출 시점의 ctx 값(관측·인증 관통 검증용)을 기록한다.
type ctxCapture struct {
	claims  *jwtresolver.CustomClaims
	traceID string
}

func (c *ctxCapture) capture(ctx context.Context) {
	if claims, ok := middleware.GetClaims(ctx); ok {
		c.claims = claims
	}
	if traceID, ok := ctx.Value(mw.CtxKeyTraceID).(string); ok {
		c.traceID = traceID
	}
}

// ---- fakes: 업스트림 gRPC 클라이언트 3종 ----

type fakeReviewClient struct {
	reviews       []*reviewGrpc.Review
	reviewsErr    error
	companyScores map[string]*reviewGrpc.CompanyScore
	scoresErr     error

	capture    ctxCapture
	gotReviews *reviewGrpc.GetCompanyReviewsRequest
}

func (f *fakeReviewClient) GetCompanyScores(ctx context.Context, in *reviewGrpc.GetCompanyScoresRequest, opts ...grpc.CallOption) (*reviewGrpc.GetCompanyScoresResponse, error) {
	if f.scoresErr != nil {
		return nil, f.scoresErr
	}
	return &reviewGrpc.GetCompanyScoresResponse{CompanyScores: f.companyScores}, nil
}

func (f *fakeReviewClient) GetCompanyReviews(ctx context.Context, in *reviewGrpc.GetCompanyReviewsRequest, opts ...grpc.CallOption) (*reviewGrpc.GetCompanyReviewsResponse, error) {
	f.capture.capture(ctx)
	f.gotReviews = in
	if f.reviewsErr != nil {
		return nil, f.reviewsErr
	}
	return &reviewGrpc.GetCompanyReviewsResponse{Reviews: f.reviews}, nil
}

type fakePostingClient struct {
	jobPostings  []*postingGrpc.JobPostingRes
	count        int64
	detail       *postingGrpc.JobPostingDetailRes // nil이면 IsExist=false
	categories   *postingGrpc.CategoriesResponse
	skills       *postingGrpc.SkillsResponse
	companies    *postingGrpc.CompaniesResponse
	err          error
	postingsById []*postingGrpc.JobPostingRes
	gotJobReq    *postingGrpc.JobPostingsRequest
	gotCompanies *postingGrpc.CompaniesRequest
	gotDetailReq *postingGrpc.JobPostingDetailRequest
	capture      ctxCapture
}

func (f *fakePostingClient) JobPostings(ctx context.Context, in *postingGrpc.JobPostingsRequest, opts ...grpc.CallOption) (*postingGrpc.JobPostingsResponse, error) {
	f.capture.capture(ctx)
	f.gotJobReq = in
	if f.err != nil {
		return nil, f.err
	}
	return &postingGrpc.JobPostingsResponse{JobPostings: f.jobPostings}, nil
}

func (f *fakePostingClient) CountJobPostings(ctx context.Context, in *postingGrpc.JobPostingsRequest, opts ...grpc.CallOption) (*postingGrpc.CountJobPostingsResponse, error) {
	f.gotJobReq = in
	if f.err != nil {
		return nil, f.err
	}
	return &postingGrpc.CountJobPostingsResponse{Count: f.count}, nil
}

func (f *fakePostingClient) JobPostingDetail(ctx context.Context, in *postingGrpc.JobPostingDetailRequest, opts ...grpc.CallOption) (*postingGrpc.JobPostingDetailResponse, error) {
	f.gotDetailReq = in
	if f.err != nil {
		return nil, f.err
	}
	return &postingGrpc.JobPostingDetailResponse{IsExist: f.detail != nil, Detail: f.detail}, nil
}

func (f *fakePostingClient) Categories(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*postingGrpc.CategoriesResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.categories, nil
}

func (f *fakePostingClient) Skills(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*postingGrpc.SkillsResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.skills, nil
}

func (f *fakePostingClient) JobPostingsById(ctx context.Context, in *postingGrpc.JobPostingsByIdRequest, opts ...grpc.CallOption) (*postingGrpc.JobPostingsResponse, error) {
	return &postingGrpc.JobPostingsResponse{JobPostings: f.postingsById}, nil
}

func (f *fakePostingClient) Companies(ctx context.Context, in *postingGrpc.CompaniesRequest, opts ...grpc.CallOption) (*postingGrpc.CompaniesResponse, error) {
	f.gotCompanies = in
	if f.err != nil {
		return nil, f.err
	}
	return f.companies, nil
}

type fakeScrapJobClient struct {
	scrapJobs    []*scrapGrpc.ScrapJob // GetScrapJobs·GetScrapJobsById·GetUntaggedScrapJobs 공용
	byIdErr      error                 // GetScrapJobsById 전용 에러 (보강 열화 경로 테스트용)
	isExisted    bool
	tags         []string
	err          error
	gotGet       *scrapGrpc.GetScrapJobsRequest
	gotAdd       *scrapGrpc.AddScrapJobRequest
	gotRemove    *scrapGrpc.RemoveScrapJobRequest
	gotAddTag    *scrapGrpc.AddTagRequest
	gotRemoveTag *scrapGrpc.RemoveTagRequest
	capture      ctxCapture
}

func (f *fakeScrapJobClient) GetScrapJobs(ctx context.Context, in *scrapGrpc.GetScrapJobsRequest, opts ...grpc.CallOption) (*scrapGrpc.GetScrapJobsResponse, error) {
	f.capture.capture(ctx)
	f.gotGet = in
	if f.err != nil {
		return nil, f.err
	}
	return &scrapGrpc.GetScrapJobsResponse{ScrapJobs: f.scrapJobs}, nil
}

func (f *fakeScrapJobClient) AddScrapJob(ctx context.Context, in *scrapGrpc.AddScrapJobRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	f.gotAdd = in
	if f.err != nil {
		return nil, f.err
	}
	return &emptypb.Empty{}, nil
}

func (f *fakeScrapJobClient) RemoveScrapJob(ctx context.Context, in *scrapGrpc.RemoveScrapJobRequest, opts ...grpc.CallOption) (*scrapGrpc.IsExistedResponse, error) {
	f.gotRemove = in
	if f.err != nil {
		return nil, f.err
	}
	return &scrapGrpc.IsExistedResponse{IsExisted: f.isExisted}, nil
}

func (f *fakeScrapJobClient) AddTag(ctx context.Context, in *scrapGrpc.AddTagRequest, opts ...grpc.CallOption) (*scrapGrpc.IsExistedResponse, error) {
	f.gotAddTag = in
	if f.err != nil {
		return nil, f.err
	}
	return &scrapGrpc.IsExistedResponse{IsExisted: f.isExisted}, nil
}

func (f *fakeScrapJobClient) RemoveTag(ctx context.Context, in *scrapGrpc.RemoveTagRequest, opts ...grpc.CallOption) (*scrapGrpc.IsExistedResponse, error) {
	f.gotRemoveTag = in
	if f.err != nil {
		return nil, f.err
	}
	return &scrapGrpc.IsExistedResponse{IsExisted: f.isExisted}, nil
}

func (f *fakeScrapJobClient) GetScrapTags(ctx context.Context, in *scrapGrpc.GetScrapTagsRequest, opts ...grpc.CallOption) (*scrapGrpc.GetScrapTagsResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &scrapGrpc.GetScrapTagsResponse{Tags: f.tags}, nil
}

func (f *fakeScrapJobClient) GetScrapJobsById(ctx context.Context, in *scrapGrpc.GetScrapJobsByIdRequest, opts ...grpc.CallOption) (*scrapGrpc.GetScrapJobsResponse, error) {
	if f.byIdErr != nil {
		return nil, f.byIdErr
	}
	return &scrapGrpc.GetScrapJobsResponse{ScrapJobs: f.scrapJobs}, nil
}

func (f *fakeScrapJobClient) GetUntaggedScrapJobs(ctx context.Context, in *scrapGrpc.GetUntaggedScrapJobsRequest, opts ...grpc.CallOption) (*scrapGrpc.GetScrapJobsResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &scrapGrpc.GetScrapJobsResponse{ScrapJobs: f.scrapJobs}, nil
}

type fakeMatchJobClient struct {
	findRes   *scrapGrpc.FindMatchJobResponse
	isSuccess bool
	err       error
	gotAdd    *scrapGrpc.AddConditionRequest
	gotUpdate *scrapGrpc.UpdateConditionRequest
	gotDelete *scrapGrpc.DeleteConditionRequest
	gotAgree  *scrapGrpc.UpdateAgreeToMailRequest
	capture   ctxCapture
}

func (f *fakeMatchJobClient) FindMatchJob(ctx context.Context, in *scrapGrpc.FindMatchJobRequest, opts ...grpc.CallOption) (*scrapGrpc.FindMatchJobResponse, error) {
	f.capture.capture(ctx)
	if f.err != nil {
		return nil, f.err
	}
	return f.findRes, nil
}

func (f *fakeMatchJobClient) AddCondition(ctx context.Context, in *scrapGrpc.AddConditionRequest, opts ...grpc.CallOption) (*scrapGrpc.IsSuccessResponse, error) {
	f.gotAdd = in
	if f.err != nil {
		return nil, f.err
	}
	return &scrapGrpc.IsSuccessResponse{IsSuccess: f.isSuccess}, nil
}

func (f *fakeMatchJobClient) UpdateCondition(ctx context.Context, in *scrapGrpc.UpdateConditionRequest, opts ...grpc.CallOption) (*scrapGrpc.IsSuccessResponse, error) {
	f.gotUpdate = in
	if f.err != nil {
		return nil, f.err
	}
	return &scrapGrpc.IsSuccessResponse{IsSuccess: f.isSuccess}, nil
}

func (f *fakeMatchJobClient) DeleteCondition(ctx context.Context, in *scrapGrpc.DeleteConditionRequest, opts ...grpc.CallOption) (*scrapGrpc.IsSuccessResponse, error) {
	f.gotDelete = in
	if f.err != nil {
		return nil, f.err
	}
	return &scrapGrpc.IsSuccessResponse{IsSuccess: f.isSuccess}, nil
}

func (f *fakeMatchJobClient) UpdateAgreeToMail(ctx context.Context, in *scrapGrpc.UpdateAgreeToMailRequest, opts ...grpc.CallOption) (*scrapGrpc.IsSuccessResponse, error) {
	f.gotAgree = in
	if f.err != nil {
		return nil, f.err
	}
	return &scrapGrpc.IsSuccessResponse{IsSuccess: f.isSuccess}, nil
}

// ---- 서버 하네스 ----

type fakes struct {
	posting  *fakePostingClient
	scrapJob *fakeScrapJobClient
	matchJob *fakeMatchJobClient
	review   *fakeReviewClient
}

func newFakes() *fakes {
	return &fakes{
		posting:  &fakePostingClient{},
		scrapJob: &fakeScrapJobClient{},
		matchJob: &fakeMatchJobClient{},
		review:   &fakeReviewClient{},
	}
}

// newStrictTestServer는 app.go의 라우터 구성을 그대로 재현한다:
// ROOT_PATH prefix subrouter + 미들웨어 체인(otelmux·TraceContextLoggerMW·SetClaimsMW·명세 검증)
// 위에 전체 생성 배선(NewStrictHTTPHandler + StrictServer)을 얹는다.
func newStrictTestServer(t *testing.T, f *fakes) (*httptest.Server, *tracetest.SpanRecorder) {
	t.Helper()

	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))

	specValidationMW, err := controller.NewSpecValidationMW(testPrefix)
	require.NoError(t, err)

	rootRouter := mux.NewRouter().PathPrefix(testPrefix).Subrouter()
	rootRouter.Use(
		otelmux.Middleware("api-composer-test", otelmux.WithTracerProvider(tracerProvider)),
		middleware.TraceContextLoggerMW(),
		middleware.SetClaimsMW(testZitadel.NewResolver(t)),
		specValidationMW,
	)

	metaController, err := controller.NewMetaStrictController()
	require.NoError(t, err)

	postingService := posting.NewPostingService(f.posting, f.scrapJob, f.review)
	strictServer := controller.NewStrictServer(
		controller.NewJobPostingStrictController(postingService),
		controller.NewScrapJobStrictController(userinfo.NewScrapJobService(f.scrapJob, f.posting, f.review)),
		controller.NewMatchJobStrictController(userinfo.NewMatchJobService(f.matchJob)),
		controller.NewReviewStrictController(review.NewReviewService(f.review)),
		metaController,
	)
	controller.NewStrictHTTPHandler(strictServer, rootRouter)

	server := httptest.NewServer(rootRouter)
	t.Cleanup(server.Close)
	return server, spanRecorder
}

func newToken(t *testing.T, userID string, authorities []string) string {
	t.Helper()
	return testZitadel.IssueAccessToken(userID, authorities)
}

// doRequest — token 빈 문자열이면 비로그인, body nil이면 본문 없음.
func doRequest(t *testing.T, method, url, token string, body any) (*http.Response, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		if raw, ok := body.(string); ok {
			reader = bytes.NewBufferString(raw)
		} else {
			encoded, err := json.Marshal(body)
			require.NoError(t, err)
			reader = bytes.NewBuffer(encoded)
		}
	}
	req, err := http.NewRequest(method, url, reader)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, respBody
}
