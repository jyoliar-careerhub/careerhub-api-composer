package app

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/common/user_authority"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/controller"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/jwtresolver"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/middleware"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/posting"
	postingGrpc "github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/posting/restapi_grpc"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/review"
	reviewGrpc "github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/review/restapi_grpc"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/userinfo"
	userinfoGrpc "github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/userinfo/restapi_grpc"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/vars"
	"github.com/jae2274/goutils/llog"
	"github.com/jae2274/goutils/mw"
	"github.com/jae2274/goutils/mw/grpcmw"
	"github.com/jae2274/goutils/mw/httpmw"
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
	llog.SetDefaultContextData(ctxKeyTraceID)

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	llog.SetMetadata("hostname", hostname)

	return nil
}

func createConn(ctx context.Context, endpoint string) (*grpc.ClientConn, error) {
	return grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainStreamInterceptor(grpcmw.SetTraceIdStreamMW()),
		grpc.WithChainUnaryInterceptor(grpcmw.SetTraceIdUnaryMW()),
	)
}

func Run(mainCtx context.Context) {

	err := initLogger(mainCtx)
	checkErr(mainCtx, err)
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

	rootRouter.Use(httpmw.SetTraceIdMW(), middleware.SetClaimsMW(jr))

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
