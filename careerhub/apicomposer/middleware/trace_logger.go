package middleware

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jae2274/goutils/mw"
	"go.opentelemetry.io/otel/trace"
)

// SpanIDCtxKey는 ctx에 span_id를 박을 때 사용하는 키.
// goutils가 trace_id ctx key를 mw.CtxKeyTraceID("trace_id")로 정의하고 있으므로
// span_id도 동일하게 string literal 키로 둔다 (llog.SetDefaultContextData가 string key 기반).
const SpanIDCtxKey = "span_id"

// TraceContextLoggerMW는 OTel span context의 TraceID/SpanID를
// goutils llog가 자동 추출하도록 ctx에 string 키로 박는 미들웨어.
//
// 전제: 본 미들웨어는 otelmux.Middleware(...) 다음에 체인되어야 한다.
// otelmux가 incoming `traceparent` 헤더를 파싱하여 ctx에 span을 주입한 뒤,
// 본 미들웨어가 span에서 TraceID/SpanID hex 문자열을 추출하여
// goutils 컨벤션의 ctx 키(`trace_id`, `span_id`)에 박는다.
//
// 결정 ② (Phase 4-B 사전 조사 §2.5): goutils trace_id를 OTel TraceID로 교체.
// 기존 httpmw.SetTraceIdMW가 발급하던 UUID v4는 더 이상 사용하지 않는다.
//
// span context가 invalid한 경우(otelmux 누락·traceparent 없음 + 샘플링 0 등)는
// ctx를 그대로 통과시킨다. 이 경우 로그에 trace_id/span_id가 부착되지 않는다.
func TraceContextLoggerMW() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			sc := trace.SpanFromContext(ctx).SpanContext()
			if sc.IsValid() {
				ctx = context.WithValue(ctx, mw.CtxKeyTraceID, sc.TraceID().String())
				ctx = context.WithValue(ctx, SpanIDCtxKey, sc.SpanID().String())
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
	}
}
