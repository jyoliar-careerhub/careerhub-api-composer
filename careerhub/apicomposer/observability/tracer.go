// Package observability는 OTel SDK 초기화·shutdown을 담당한다.
//
// 트레이서 init만 처리 (Phase 4-B). logs/metrics는 별도 Phase에서 추가될 수 있다.
package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// InitTracer는 글로벌 TracerProvider와 TextMapPropagator를 초기화한다.
//
// 설정은 환경변수로부터 읽는다 (manifest에서 주입, Phase 4-B 사전 조사 §6 참조):
//   - OTEL_EXPORTER_OTLP_ENDPOINT: Alloy(careerhub 텐넌트 전용)의 OTLP gRPC 엔드포인트
//   - OTEL_EXPORTER_OTLP_PROTOCOL: "grpc" (본 함수는 grpc만 지원)
//   - OTEL_SERVICE_NAME: K8s Deployment label `app` (`api-composer` 등)
//   - OTEL_RESOURCE_ATTRIBUTES: `deployment.environment=<namespace>` 등 추가 속성
//
// X-Scope-OrgID 헤더는 본 함수에서 박지 않는다 — Alloy가 단일 텐넌트 하드코딩으로 박음.
//
// 샘플러는 ParentBased(TraceIDRatioBased(1.0)) — 초기 100% 샘플링. 상위에서
// 샘플링이 결정되었으면 그것을 따르고(otelmux가 W3C TraceContext의 sampled flag 존중),
// 새 root span은 100% 샘플링. 트래픽 증가 후 ratio 조정 시 본 함수 인자화 검토.
//
// 반환값:
//   - shutdown: graceful shutdown 함수. main에서 defer로 호출하여 batched span flush
//   - err: 초기화 실패 시 (exporter 생성 실패 등)
func InitTracer(ctx context.Context) (shutdown func(context.Context) error, err error) {
	// Resource 구성. OTEL_SERVICE_NAME, OTEL_RESOURCE_ATTRIBUTES는 WithFromEnv가 자동 흡수.
	res, err := resource.New(ctx,
		resource.WithFromEnv(),    // OTEL_SERVICE_NAME, OTEL_RESOURCE_ATTRIBUTES
		resource.WithTelemetrySDK(), // telemetry.sdk.{name,language,version}
		resource.WithProcess(),    // process.pid, process.executable.*
		resource.WithContainer(),  // container.id (K8s에서 자동 감지)
	)
	if err != nil {
		return nil, fmt.Errorf("observability: resource init failed: %w", err)
	}

	// OTLP gRPC exporter. OTEL_EXPORTER_OTLP_ENDPOINT 자동 흡수.
	// otlptracegrpc는 기본 insecure(WithInsecure 명시 없이도 endpoint scheme이 http://면 평문).
	// manifest의 endpoint가 http:// 형태라 평문 전송 — in-cluster 통신이라 OK.
	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("observability: otlp exporter init failed: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(1.0))),
	)

	otel.SetTracerProvider(tp)
	// W3C TraceContext + Baggage. NGF가 traceparent 헤더로 propagate 중.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}
