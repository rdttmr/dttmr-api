package telemetry_test

import (
	"context"
	"net"
	"slices"
	"sync"
	"testing"
	"time"

	"git.dittmar.dev/robin/dttmr-api/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	collmetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	colltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/grpc"
)

// fakeCollector is an in-process OTLP/gRPC receiver that records everything it gets.
type fakeCollector struct {
	colltracepb.UnimplementedTraceServiceServer

	mu      sync.Mutex
	traces  []*colltracepb.ExportTraceServiceRequest
	metrics []*collmetricpb.ExportMetricsServiceRequest
}

func (c *fakeCollector) Export(_ context.Context, req *colltracepb.ExportTraceServiceRequest) (*colltracepb.ExportTraceServiceResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.traces = append(c.traces, req)
	return &colltracepb.ExportTraceServiceResponse{}, nil
}

type metricsServer struct {
	collmetricpb.UnimplementedMetricsServiceServer
	c *fakeCollector
}

func (m metricsServer) Export(_ context.Context, req *collmetricpb.ExportMetricsServiceRequest) (*collmetricpb.ExportMetricsServiceResponse, error) {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()
	m.c.metrics = append(m.c.metrics, req)
	return &collmetricpb.ExportMetricsServiceResponse{}, nil
}

func startCollector(t *testing.T) (*fakeCollector, string) {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	c := &fakeCollector{}
	srv := grpc.NewServer()
	colltracepb.RegisterTraceServiceServer(srv, c)
	collmetricpb.RegisterMetricsServiceServer(srv, metricsServer{c: c})

	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	return c, lis.Addr().String()
}

func restoreGlobals(t *testing.T) {
	t.Helper()
	tp, mp, prop := otel.GetTracerProvider(), otel.GetMeterProvider(), otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(tp)
		otel.SetMeterProvider(mp)
		otel.SetTextMapPropagator(prop)
	})
}

func resourceAttr(res *resourcepb.Resource, key string) string {
	for _, kv := range res.GetAttributes() {
		if kv.GetKey() == key {
			return kv.GetValue().GetStringValue()
		}
	}
	return ""
}

func testConfig(endpoint string) telemetry.Config {
	return telemetry.Config{
		ServiceName:    "dttmr-api-test",
		ServiceVersion: "1.2.3",
		Endpoint:       endpoint,
		Environment:    "test",
	}
}

func TestInit_SetsGlobals(t *testing.T) {
	restoreGlobals(t)
	_, endpoint := startCollector(t)

	shutdown, err := telemetry.Init(t.Context(), testConfig(endpoint))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(ctx)
	})

	if shutdown == nil {
		t.Fatal("Init returned nil shutdown func")
	}
	if _, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); !ok {
		t.Errorf("global tracer provider = %T, want *sdktrace.TracerProvider", otel.GetTracerProvider())
	}
	if _, ok := otel.GetMeterProvider().(*sdkmetric.MeterProvider); !ok {
		t.Errorf("global meter provider = %T, want *sdkmetric.MeterProvider", otel.GetMeterProvider())
	}

	fields := otel.GetTextMapPropagator().Fields()
	for _, want := range []string{"traceparent", "tracestate", "baggage"} {
		if !slices.Contains(fields, want) {
			t.Errorf("propagator fields %v missing %q", fields, want)
		}
	}

	// Round-trip a trace context through the global propagator.
	ctx, span := otel.Tracer("test").Start(t.Context(), "propagate")
	defer span.End()
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	if carrier.Get("traceparent") == "" {
		t.Error("traceparent header was not injected")
	}
}

func TestInit_ExportsTelemetryOnShutdown(t *testing.T) {
	restoreGlobals(t)
	collector, endpoint := startCollector(t)

	shutdown, err := telemetry.Init(t.Context(), testConfig(endpoint))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	_, span := otel.Tracer("test").Start(t.Context(), "test-span")
	span.End()

	counter, err := otel.Meter("test").Int64Counter("test.counter")
	if err != nil {
		t.Fatalf("create counter: %v", err)
	}
	counter.Add(t.Context(), 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	collector.mu.Lock()
	defer collector.mu.Unlock()

	// Spans: shutdown must flush the batch span processor.
	var gotSpan bool
	for _, req := range collector.traces {
		for _, rs := range req.GetResourceSpans() {
			assertResource(t, rs.GetResource())
			for _, ss := range rs.GetScopeSpans() {
				for _, s := range ss.GetSpans() {
					if s.GetName() == "test-span" {
						gotSpan = true
					}
				}
			}
		}
	}
	if !gotSpan {
		t.Error("span \"test-span\" was not exported before shutdown returned")
	}

	// Metrics: shutdown performs a final collect/export of the periodic reader.
	var gotMetric bool
	for _, req := range collector.metrics {
		for _, rm := range req.GetResourceMetrics() {
			assertResource(t, rm.GetResource())
			for _, sm := range rm.GetScopeMetrics() {
				for _, m := range sm.GetMetrics() {
					if m.GetName() == "test.counter" {
						gotMetric = true
					}
				}
			}
		}
	}
	if !gotMetric {
		t.Error("metric \"test.counter\" was not exported before shutdown returned")
	}
}

func assertResource(t *testing.T, res *resourcepb.Resource) {
	t.Helper()
	if got := resourceAttr(res, "service.name"); got != "dttmr-api-test" {
		t.Errorf("resource service.name = %q, want %q", got, "dttmr-api-test")
	}
	if got := resourceAttr(res, "service.version"); got != "1.2.3" {
		t.Errorf("resource service.version = %q, want %q", got, "1.2.3")
	}
	if got := resourceAttr(res, "deployment.environment"); got != "test" {
		t.Errorf("resource deployment.environment = %q, want %q", got, "test")
	}
}
