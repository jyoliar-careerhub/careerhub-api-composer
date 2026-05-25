package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/common/user_authority"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/controller"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/jwtresolver"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/middleware"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/observability"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/posting"
	postingGrpc "github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/posting/restapi_grpc"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/review"
	reviewGrpc "github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/review/restapi_grpc"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/userinfo"
	userinfoGrpc "github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/userinfo/restapi_grpc"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/vars"
	"github.com/jae2274/goutils/llog"
	"github.com/jae2274/goutils/mw"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	app         = "api-composer"
	serviceName = "careerhub"

	ctxKeyTraceID = string(mw.CtxKeyTraceID)

	// needAuthority = "AUTHORITY_CAREERHUB_USER"
)

func initLogger(ctx context.Context) error {
	llog.SetMetadata("service", serviceName)
	llog.SetMetadata("app", app)
	// OTel TraceContextLoggerMW가 ctx에 박는 키를 로그에 자동 부착하도록 등록.
	// 기존 goutils httpmw.SetTraceIdMW가 발급하던 UUID 대신 OTel TraceID(32 hex)가 들어감.
	llog.SetDefaultContextData(ctxKeyTraceID)
	llog.SetDefaultContextData(middleware.SpanIDCtxKey)

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	llog.SetMetadata("hostname", hostname)

	return nil
}

func createConn(ctx context.Context, endpoint string) (*grpc.ClientConn, error) {
	// otelgrpc StatsHandler가 unary/stream 양쪽 + W3C TraceContext propagation을 자동 처리.
	// 기존 grpcmw.SetTraceId{Unary,Stream}MW는 폐기 (Phase 4-B 사전 조사 §2.5).
	return grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
}

func Run(mainCtx context.Context) {

	err := initLogger(mainCtx)
	checkErr(mainCtx, err)

	shutdownTracer, err := observability.InitTracer(mainCtx)
	checkErr(mainCtx, err)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if shutdownErr := shutdownTracer(shutdownCtx); shutdownErr != nil {
			llog.LogErr(mainCtx, shutdownErr)
		}
	}()

	llog.Info(mainCtx, "Start Application")

	envVars, err := vars.Variables()
	checkErr(mainCtx, err)

	conn, err := createConn(mainCtx, envVars.PostingGrpcEndpoint)
	checkErr(mainCtx, err)
	postingClient := postingGrpc.NewRestApiGrpcClient(conn)

	conn, err = createConn(mainCtx, envVars.UserinfoGrpcEndpoint)
	checkErr(mainCtx, err)
	matchJobClient := userinfoGrpc.NewMatchJobGrpcClient(conn)
	scrapJobClient := userinfoGrpc.NewScrapJobGrpcClient(conn)

	conn, err = createConn(mainCtx, envVars.ReviewGrpcEndpoint)
	checkErr(mainCtx, err)
	reviewClient := reviewGrpc.NewReviewReaderGrpcClient(conn)

	jr := jwtresolver.NewJwtResolver(envVars.SecretKey)

	//rootRouter 설정
	rootRouter := mux.NewRouter().PathPrefix(envVars.RootPath).Subrouter()

	// 미들웨어 체인 순서:
	//   1) otelmux: incoming traceparent 헤더에서 server span 시작, ctx에 span 주입
	//   2) TraceContextLoggerMW: ctx의 OTel span에서 trace_id/span_id를 string key로 추출 — goutils llog 호환
	//   3) SetClaimsMW: JWT 파싱 (기존)
	rootRouter.Use(
		otelmux.Middleware(app),
		middleware.TraceContextLoggerMW(),
		middleware.SetClaimsMW(jr),
	)

	postingService := posting.NewPostingService(postingClient, scrapJobClient, reviewClient)
	matchJobService := userinfo.NewMatchJobService(matchJobClient)
	scrapJobService := userinfo.NewScrapJobService(scrapJobClient, postingClient, reviewClient)
	reviewService := review.NewReviewService(reviewClient)

	jobPostingRouter := rootRouter.NewRoute().Subrouter()
	// jobPostingRouter.Use(middleware.CheckHasAuthority(user_authority.AuthorityReadJobPosting))
	controller.NewJobPostingController(postingService).RegisterRoutes(jobPostingRouter)

	//userinfoRouter 설정
	userinfoRouter := rootRouter.PathPrefix("/my").Subrouter()
	scrapJobRouter := userinfoRouter.NewRoute().Subrouter()
	// scrapJobRouter.Use(middleware.CheckHasAuthority(user_authority.AuthorityScrapJob))
	controller.NewScrapJobController(scrapJobService).RegisterRoutes(scrapJobRouter)

	//matchJobRouter 설정
	matchJobRouter := userinfoRouter.NewRoute().Subrouter()
	matchJobRouter.Use(middleware.CheckHasAuthority(user_authority.AuthorityMatchJob))
	controller.NewMatchJobController(matchJobService).RegisterRoutes(matchJobRouter)

	//reviewRouter 설정
	reviewRouter := rootRouter.NewRoute().Subrouter()
	// reviewRouter.Use(middleware.CheckHasAuthority(user_authority.AuthorityReadReview))
	controller.NewReviewController(reviewService).RegisterRoutes(reviewRouter)

	var allowOrigins []string
	if envVars.AccessControlAllowOrigin != nil {
		allowOrigins = append(allowOrigins, *envVars.AccessControlAllowOrigin)
	}
	originsOk := handlers.AllowedOrigins(allowOrigins)
	headersOk := handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Authorization"})
	methodsOk := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "DELETE", "OPTIONS"})

	llog.Msg("Start composed api server").Level(llog.INFO).Data("port", envVars.ApiPort).Data("rootPath", envVars.RootPath).Log(mainCtx)

	err = http.ListenAndServe(fmt.Sprintf(":%d", envVars.ApiPort), handlers.CORS(originsOk, headersOk, methodsOk)(rootRouter))
	checkErr(mainCtx, err)
}

func checkErr(ctx context.Context, err error) {
	if err != nil {
		llog.LogErr(ctx, err)
		os.Exit(1)
	}
}
