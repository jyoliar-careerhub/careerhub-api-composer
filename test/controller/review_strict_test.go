package controller

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/controller"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/jwtresolver"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/middleware"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/review"
	reviewGrpc "github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/review/restapi_grpc"
	"github.com/jae2274/goutils/mw"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc"
)

const testSecret = "test-secret-key"

// fakeReviewClient는 업스트림 review gRPC를 대신한다.
// 핸들러 ctx가 배선·미들웨어를 관통해 서비스까지 오는지 확인하기 위해 ctx의 값들을 기록한다.
type fakeReviewClient struct {
	reviews []*reviewGrpc.Review
	err     error

	called     bool
	gotReq     *reviewGrpc.GetCompanyReviewsRequest
	gotClaims  *jwtresolver.CustomClaims
	gotTraceID string
}

func (f *fakeReviewClient) GetCompanyScores(ctx context.Context, in *reviewGrpc.GetCompanyScoresRequest, opts ...grpc.CallOption) (*reviewGrpc.GetCompanyScoresResponse, error) {
	return &reviewGrpc.GetCompanyScoresResponse{}, nil
}

func (f *fakeReviewClient) GetCompanyReviews(ctx context.Context, in *reviewGrpc.GetCompanyReviewsRequest, opts ...grpc.CallOption) (*reviewGrpc.GetCompanyReviewsResponse, error) {
	f.called = true
	f.gotReq = in
	if claims, ok := middleware.GetClaims(ctx); ok {
		f.gotClaims = claims
	}
	if traceID, ok := ctx.Value(mw.CtxKeyTraceID).(string); ok {
		f.gotTraceID = traceID
	}
	if f.err != nil {
		return nil, f.err
	}
	return &reviewGrpc.GetCompanyReviewsResponse{Reviews: f.reviews}, nil
}

// reviewOnlyStrictServer는 배선 전 임시 어댑터다: 구현된 review 메서드만 위임하고,
// 나머지 연산은 임베딩된 nil 인터페이스라 호출되면 패닉한다(테스트 한정 — 프로덕션 배선은 일괄 전환 시점에).
type reviewOnlyStrictServer struct {
	api.StrictServerInterface
	reviewCtrl *controller.ReviewStrictController
}

func (s reviewOnlyStrictServer) GetCompanyReviews(ctx context.Context, req api.GetCompanyReviewsRequestObject) (api.GetCompanyReviewsResponseObject, error) {
	return s.reviewCtrl.GetCompanyReviews(ctx, req)
}

// newStrictTestServer는 app.go의 라우터 구성을 그대로 재현한다:
// ROOT_PATH prefix subrouter + 미들웨어 체인(otelmux·TraceContextLoggerMW·SetClaimsMW)
// 위에 생성 배선(NewStrictHTTPHandler)을 얹는다.
func newStrictTestServer(t *testing.T, fake *fakeReviewClient) (*httptest.Server, *tracetest.SpanRecorder) {
	t.Helper()

	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))

	rootRouter := mux.NewRouter().PathPrefix("/api").Subrouter()
	rootRouter.Use(
		otelmux.Middleware("api-composer-test", otelmux.WithTracerProvider(tracerProvider)),
		middleware.TraceContextLoggerMW(),
		middleware.SetClaimsMW(jwtresolver.NewJwtResolver(testSecret)),
	)

	strictServer := reviewOnlyStrictServer{
		reviewCtrl: controller.NewReviewStrictController(review.NewReviewService(fake)),
	}
	controller.NewStrictHTTPHandler(strictServer, rootRouter)

	server := httptest.NewServer(rootRouter)
	t.Cleanup(server.Close)
	return server, spanRecorder
}

func getJSON(t *testing.T, url string, token string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, body
}

func TestReviewStrict_Success(t *testing.T) {
	unixMilli := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC).UnixMilli()
	fake := &fakeReviewClient{reviews: []*reviewGrpc.Review{{
		Score:            4,
		Summary:          "좋은 회사",
		EmploymentStatus: true,
		ReviewUserId:     "user-1",
		JobType:          "백엔드",
		UnixMilli:        unixMilli,
	}}}
	server, spanRecorder := newStrictTestServer(t, fake)

	resp, body := getJSON(t, server.URL+"/api/company-review/reviews?companyName=acme&page=2&size=5", "")

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")

	var reviews []api.Review
	require.NoError(t, json.Unmarshal(body, &reviews))
	require.Len(t, reviews, 1)
	assert.Equal(t, int32(4), reviews[0].Score)
	assert.Equal(t, "좋은 회사", reviews[0].Summary)
	assert.True(t, reviews[0].EmploymentStatus)
	assert.Equal(t, "user-1", reviews[0].ReviewUserId)
	assert.Equal(t, "백엔드", reviews[0].JobType)
	assert.Equal(t, unixMilli, reviews[0].Date.UnixMilli())

	// 성공 계약 불변: 기존 핸들러와 같은 페이지네이션 변환(offset = page*size)
	require.NotNil(t, fake.gotReq)
	assert.Equal(t, "acme", fake.gotReq.CompanyName)
	assert.Equal(t, int64(10), fake.gotReq.Offset)
	assert.Equal(t, int64(5), fake.gotReq.Limit)

	// 관측 유지: otelmux 스팬이 생성 배선 라우트에서도 기록된다
	require.NotEmpty(t, spanRecorder.Ended())
	// TraceContextLoggerMW가 박은 OTel TraceID(32 hex)가 핸들러 ctx까지 관통한다
	assert.Len(t, fake.gotTraceID, 32)
}

func TestReviewStrict_DefaultPaging(t *testing.T) {
	fake := &fakeReviewClient{}
	server, _ := newStrictTestServer(t, fake)

	resp, _ := getJSON(t, server.URL+"/api/company-review/reviews?companyName=acme", "")

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotNil(t, fake.gotReq)
	assert.Equal(t, int64(0), fake.gotReq.Offset)
	assert.Equal(t, int64(10), fake.gotReq.Limit)
}

func TestReviewStrict_ErrorContract(t *testing.T) {
	cases := []struct {
		name        string
		query       string
		fakeErr     error
		wantStatus  int
		wantMessage string // 빈 문자열이면 message 존재만 확인
	}{
		{name: "음수 page", query: "?companyName=acme&page=-1", wantStatus: 400, wantMessage: "Invalid page"},
		{name: "음수 size", query: "?companyName=acme&size=-1", wantStatus: 400, wantMessage: "Invalid page size"},
		{name: "빈 companyName", query: "?companyName=", wantStatus: 400, wantMessage: "companyName is required"},
		{name: "companyName 누락(바인딩)", query: "?page=0", wantStatus: 400},
		{name: "비정수 page(바인딩)", query: "?companyName=acme&page=abc", wantStatus: 400},
		{name: "업스트림 오류", query: "?companyName=acme", fakeErr: context.DeadlineExceeded, wantStatus: 500, wantMessage: "Internal Server Error"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeReviewClient{err: tc.fakeErr}
			server, _ := newStrictTestServer(t, fake)

			resp, body := getJSON(t, server.URL+"/api/company-review/reviews"+tc.query, "")

			require.Equal(t, tc.wantStatus, resp.StatusCode)
			// 에러 계약: text/plain이 아니라 구조화 JSON(ErrorResponse)
			assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
			var errResp api.ErrorResponse
			require.NoError(t, json.Unmarshal(body, &errResp), "body: %s", body)
			require.NotEmpty(t, errResp.Message)
			if tc.wantMessage != "" {
				assert.Equal(t, tc.wantMessage, errResp.Message)
			}
		})
	}
}

func TestReviewStrict_Auth(t *testing.T) {
	jr := jwtresolver.NewJwtResolver(testSecret)

	t.Run("유효 토큰이면 claims가 핸들러 ctx까지 관통한다", func(t *testing.T) {
		fake := &fakeReviewClient{}
		server, _ := newStrictTestServer(t, fake)

		tokens, err := jr.CreateToken("test-user", []string{"AUTHORITY_READ_REVIEW"}, time.Now())
		require.NoError(t, err)

		resp, _ := getJSON(t, server.URL+"/api/company-review/reviews?companyName=acme", tokens.AccessToken)

		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.NotNil(t, fake.gotClaims)
		assert.Equal(t, "test-user", fake.gotClaims.UserId)
	})

	t.Run("만료 토큰이면 미들웨어가 401로 끊는다(기존 동작 유지)", func(t *testing.T) {
		fake := &fakeReviewClient{}
		server, _ := newStrictTestServer(t, fake)

		tokens, err := jr.CreateToken("test-user", nil, time.Now().Add(-24*time.Hour))
		require.NoError(t, err)

		resp, body := getJSON(t, server.URL+"/api/company-review/reviews?companyName=acme", tokens.AccessToken)

		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.Empty(t, body)
		assert.False(t, fake.called)
	})
}
