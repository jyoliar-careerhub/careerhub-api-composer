package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
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

	// 요청 검증 미들웨어 — 내장 명세와 대조해 파라미터·본문의 형식 위반을 400으로 거부
	specValidationMW, err := controller.NewSpecValidationMW(envVars.RootPath)
	checkErr(mainCtx, err)

	// 미들웨어 체인 순서:
	//   1) otelmux: incoming traceparent 헤더에서 server span 시작, ctx에 span 주입
	//   2) TraceContextLoggerMW: ctx의 OTel span에서 trace_id/span_id를 string key로 추출 — goutils llog 호환
	//   3) SetClaimsMW: JWT 파싱 (기존)
	//   4) 명세 검증: 무효 토큰 401이 검증보다 먼저 끊기도록 SetClaimsMW 뒤에 둔다
	rootRouter.Use(
		otelmux.Middleware(app),
		middleware.TraceContextLoggerMW(),
		middleware.SetClaimsMW(jr),
		specValidationMW,
	)

	postingService := posting.NewPostingService(postingClient, scrapJobClient, reviewClient)
	matchJobService := userinfo.NewMatchJobService(matchJobClient)
	scrapJobService := userinfo.NewScrapJobService(scrapJobClient, postingClient, reviewClient)
	reviewService := review.NewReviewService(reviewClient)

	// 전 라우트가 명세 생성 배선으로 등록된다 (ADR-0018 — 손 등록 없음).
	// matchJob 연산의 권한 검사(AUTHORITY_MATCH_JOB)는 배선 내부의 권한 미들웨어가 한다.
	metaController, err := controller.NewMetaStrictController()
	checkErr(mainCtx, err)

	strictServer := controller.NewStrictServer(
		controller.NewJobPostingStrictController(postingService),
		controller.NewScrapJobStrictController(scrapJobService),
		controller.NewMatchJobStrictController(matchJobService),
		controller.NewReviewStrictController(reviewService),
		metaController,
	)
	controller.NewStrictHTTPHandler(strictServer, rootRouter)

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
