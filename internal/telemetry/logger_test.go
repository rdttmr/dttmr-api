package telemetry_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"git.dittmar.dev/robin/dttmr-api/internal/telemetry"
	"go.opentelemetry.io/otel/trace"
)

var (
	testTraceID = trace.TraceID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
	testSpanID  = trace.SpanID{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18}
)

// newTraceLogger returns a logger that writes JSON through a TraceHandler into buf.
func newTraceLogger(buf *bytes.Buffer, level slog.Level) *slog.Logger {
	base := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: level})
	return slog.New(&telemetry.TraceHandler{Handler: base})
}

func spanContext(t *testing.T) context.Context {
	t.Helper()
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    testTraceID,
		SpanID:     testSpanID,
		TraceFlags: trace.FlagsSampled,
	})
	if !sc.IsValid() {
		t.Fatal("test span context is invalid")
	}
	return trace.ContextWithSpanContext(t.Context(), sc)
}

func decode(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("decode log line %q: %v", buf.String(), err)
	}
	return m
}

func TestTraceHandler_AddsTraceAndSpanID(t *testing.T) {
	var buf bytes.Buffer
	logger := newTraceLogger(&buf, slog.LevelInfo)

	logger.InfoContext(spanContext(t), "hello", "key", "value")

	got := decode(t, &buf)
	if got["msg"] != "hello" {
		t.Errorf("msg = %v, want %q", got["msg"], "hello")
	}
	if got["key"] != "value" {
		t.Errorf("key = %v, want %q", got["key"], "value")
	}
	if got["trace_id"] != testTraceID.String() {
		t.Errorf("trace_id = %v, want %q", got["trace_id"], testTraceID.String())
	}
	if got["span_id"] != testSpanID.String() {
		t.Errorf("span_id = %v, want %q", got["span_id"], testSpanID.String())
	}
}

func TestTraceHandler_NoSpanInContext(t *testing.T) {
	var buf bytes.Buffer
	logger := newTraceLogger(&buf, slog.LevelInfo)

	logger.InfoContext(t.Context(), "no span")

	got := decode(t, &buf)
	for _, key := range []string{"trace_id", "span_id"} {
		if _, ok := got[key]; ok {
			t.Errorf("unexpected %s = %v in log without span", key, got[key])
		}
	}
}

func TestTraceHandler_RespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := newTraceLogger(&buf, slog.LevelWarn)

	logger.InfoContext(spanContext(t), "filtered")

	if buf.Len() != 0 {
		t.Errorf("expected info record to be filtered at warn level, got %q", buf.String())
	}
}

func TestTraceHandler_WithAttrsKeepsTracing(t *testing.T) {
	var buf bytes.Buffer
	logger := newTraceLogger(&buf, slog.LevelInfo).With("component", "test")

	if _, ok := logger.Handler().(*telemetry.TraceHandler); !ok {
		t.Fatalf("With() returned handler %T, want *telemetry.TraceHandler", logger.Handler())
	}

	logger.InfoContext(spanContext(t), "with attrs")

	got := decode(t, &buf)
	if got["component"] != "test" {
		t.Errorf("component = %v, want %q", got["component"], "test")
	}
	if got["trace_id"] != testTraceID.String() {
		t.Errorf("trace_id = %v, want %q", got["trace_id"], testTraceID.String())
	}
}

func TestTraceHandler_WithGroupKeepsTracing(t *testing.T) {
	var buf bytes.Buffer
	logger := newTraceLogger(&buf, slog.LevelInfo).WithGroup("req")

	if _, ok := logger.Handler().(*telemetry.TraceHandler); !ok {
		t.Fatalf("WithGroup() returned handler %T, want *telemetry.TraceHandler", logger.Handler())
	}

	logger.InfoContext(spanContext(t), "grouped", "path", "/x")

	got := decode(t, &buf)
	group, ok := got["req"].(map[string]any)
	if !ok {
		t.Fatalf("req group missing or not an object: %v", got["req"])
	}
	if group["path"] != "/x" {
		t.Errorf("req.path = %v, want %q", group["path"], "/x")
	}

	if group["trace_id"] != testTraceID.String() {
		t.Errorf("req.trace_id = %v, want %q", group["trace_id"], testTraceID.String())
	}
	if group["span_id"] != testSpanID.String() {
		t.Errorf("req.span_id = %v, want %q", group["span_id"], testSpanID.String())
	}
}
