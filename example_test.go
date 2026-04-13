package logger_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	logger "github.com/adamjohannes/log-it"
)

func ExampleNew() {
	log := logger.New(os.Stdout, logger.INFO)
	defer func() { _ = log.Sync() }()

	log.Info("server started", map[string]any{"port": 8080})
}

func ExampleLogger_With() {
	log := logger.New(os.Stdout, logger.INFO)

	// Child logger carries fields into every entry
	reqLog := log.With(map[string]any{
		"request_id": "abc-123",
		"user_id":    42,
	})

	reqLog.Info("processing order", map[string]any{"order_id": "ord-1"})
	// Output includes request_id, user_id, and order_id
}

func ExampleLogger_Infow() {
	log := logger.New(os.Stdout, logger.INFO)

	log.Infow("request handled",
		logger.String("method", "GET"),
		logger.Int("status", 200),
		logger.Float64("latency_ms", 12.5),
		logger.Bool("cached", true),
	)
}

func ExampleLogger_Infow_group() {
	log := logger.New(os.Stdout, logger.INFO)

	log.Infow("request",
		logger.Group("http",
			logger.String("method", "POST"),
			logger.Int("status", 201),
		),
		logger.Group("user",
			logger.String("id", "u-42"),
			logger.String("role", "admin"),
		),
	)
	// Output includes nested objects: "http":{"method":"POST","status":201}
}

func ExampleLogger_InfoContext() {
	type ctxKey string

	log := logger.New(os.Stdout, logger.INFO)
	log = log.WithContextExtractor(func(ctx context.Context) map[string]any {
		if v := ctx.Value(ctxKey("trace_id")); v != nil {
			return map[string]any{"trace_id": v}
		}
		return nil
	})

	ctx := context.WithValue(context.Background(), ctxKey("trace_id"), "abc-123")
	log.InfoContext(ctx, "handled request", nil)
	// Output includes trace_id extracted from context
}

func ExampleLogger_Timed() {
	log := logger.New(os.Stdout, logger.INFO)

	// Time an operation — logs duration_ms on function return
	done := log.Timed("db_query")
	defer done()

	// ... do work ...
}

func ExampleNewAsyncWriter() {
	// Fan-out to stdout and a file, buffered through async writer
	file, err := os.CreateTemp("", "app-*.log")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.Remove(file.Name()) }()

	fan := logger.NewFanOutWriter(os.Stdout, file)
	async := logger.NewAsyncWriter(fan, 4096)
	log := logger.New(async, logger.INFO)
	defer func() { _ = log.Sync() }() // flush async buffer + fsync file

	log.Info("multi-destination", nil)

	fmt.Println("dropped:", async.DroppedCount())
}

func ExampleWithServiceIdentity() {
	log := logger.New(os.Stdout, logger.INFO,
		logger.WithServiceIdentity("api-gateway", "2.1.0", "production"),
	)

	log.Info("boot", nil)
	// Every entry includes: service, version, env, host
}

func ExampleNewSlogHandler() {
	log := logger.New(os.Stdout, logger.DEBUG)

	// Use as a backend for Go's standard log/slog
	// slogLogger := slog.New(logger.NewSlogHandler(log))
	// slogLogger.Info("from slog", "user", "alice")
	_ = logger.NewSlogHandler(log)
}

func ExampleFromContext() {
	log := logger.New(os.Stdout, logger.INFO)

	// Store logger in context (typically in middleware)
	ctx := logger.WithLogger(context.Background(), log)

	// Retrieve in downstream handlers
	l := logger.FromContext(ctx)
	l.Info("from context", nil)
}

func ExampleDefault() {
	// Set a custom default logger for the whole application
	log := logger.New(os.Stderr, logger.INFO,
		logger.WithServiceIdentity("myapp", "1.0.0", "prod"),
	)
	logger.SetDefault(log)

	// Anywhere in the codebase
	logger.Default().Info("using global default", nil)
}

func ExampleWithEnvConfig() {
	// Reads LOG_LEVEL and LOG_FORMAT from environment variables
	// LOG_LEVEL=debug LOG_FORMAT=text ./myapp
	log := logger.New(os.Stdout, logger.INFO,
		logger.WithEnvConfig(),
	)

	log.Info("configured from env", nil)
}

func ExampleNop() {
	// Nop logger discards everything — useful in tests
	log := logger.Nop()
	log.Info("this goes nowhere", nil)
	log.Error("this too", nil)
}

func ExampleLogger_StdLogger() {
	log := logger.New(os.Stdout, logger.INFO)

	// Bridge to standard library's *log.Logger
	stdLog := log.StdLogger(logger.WARNING)
	_ = stdLog // use as httpServer.ErrorLog, sql.DB logger, etc.
}

// ExampleLogger_WithContextExtractor_traceID demonstrates how to
// extract OpenTelemetry-style trace and span IDs from context.Context
// using a context extractor. No OTel dependency is needed — the
// extractor pulls whatever your tracing library stores in context.
func ExampleLogger_WithContextExtractor_traceID() {
	// This simulates what OpenTelemetry or similar tracing libraries do:
	// store trace context in context.Context. Replace with real OTel calls:
	//   span := trace.SpanFromContext(ctx)
	//   traceID := span.SpanContext().TraceID().String()
	type traceCtxKey struct{}
	type traceInfo struct{ TraceID, SpanID string }

	var buf bytes.Buffer
	log := logger.New(&buf, logger.INFO)
	log = log.WithContextExtractor(func(ctx context.Context) map[string]any {
		if info, ok := ctx.Value(traceCtxKey{}).(traceInfo); ok {
			return map[string]any{
				"trace_id": info.TraceID,
				"span_id":  info.SpanID,
			}
		}
		return nil
	})

	// In your HTTP handler or gRPC interceptor:
	ctx := context.WithValue(context.Background(), traceCtxKey{}, traceInfo{
		TraceID: "abc123",
		SpanID:  "def456",
	})

	log.InfoContext(ctx, "request handled", map[string]any{"status": 200})

	// Verify trace context is present in the log output
	output := buf.String()
	hasTrace := strings.Contains(output, `"trace_id":"abc123"`) && strings.Contains(output, `"span_id":"def456"`)
	fmt.Println("trace context logged:", hasTrace)
	// Output: trace context logged: true
}

func ExampleLogfmtFormatter() {
	var buf bytes.Buffer
	log := logger.New(&buf, logger.INFO,
		logger.WithFormatter(logger.LogfmtFormatter{}),
	)

	log.Info("request handled", map[string]any{"method": "GET", "status": 200})

	output := buf.String()
	fmt.Println("has level:", strings.Contains(output, "level=INFO"))
	fmt.Println("has message:", strings.Contains(output, "message="))
	fmt.Println("has status:", strings.Contains(output, "status=200"))
	// Output:
	// has level: true
	// has message: true
	// has status: true
}

func ExampleNewFilteredWriter() {
	var infoBuf, errorBuf bytes.Buffer

	fan := logger.NewFanOutWriter(
		logger.NewFilteredWriter(&infoBuf, logger.INFO),   // INFO+ to infoBuf
		logger.NewFilteredWriter(&errorBuf, logger.ERROR), // ERROR+ to errorBuf
	)
	log := logger.New(fan, logger.DEBUG)

	log.Debug("debug msg", nil)
	log.Info("info msg", nil)
	log.Error("error msg", nil)

	infoLines := strings.Count(strings.TrimSpace(infoBuf.String()), "\n") + 1
	errorLines := strings.Count(strings.TrimSpace(errorBuf.String()), "\n") + 1

	fmt.Println("info sink entries:", infoLines)   // INFO + ERROR
	fmt.Println("error sink entries:", errorLines)  // ERROR only
	// Output:
	// info sink entries: 2
	// error sink entries: 1
}

func ExampleWithRedactFields() {
	var buf bytes.Buffer
	log := logger.New(&buf, logger.INFO,
		logger.WithRedactFields("password", "token"),
	)

	log.Info("login", map[string]any{
		"user":     "alice",
		"password": "s3cret",
		"token":    "abc-xyz",
	})

	output := buf.String()
	fmt.Println("password hidden:", strings.Contains(output, `"[REDACTED]"`))
	fmt.Println("user visible:", strings.Contains(output, `"alice"`))
	// Output:
	// password hidden: true
	// user visible: true
}

func ExampleWithMiddleware() {
	var buf bytes.Buffer

	// Middleware that adds a field to every entry
	addRegion := func(entry map[string]any) map[string]any {
		entry["region"] = "us-east-1"
		return entry
	}

	// Middleware that drops health check logs
	dropHealth := func(entry map[string]any) map[string]any {
		if msg, _ := entry["message"].(string); msg == "health check" {
			return nil
		}
		return entry
	}

	log := logger.New(&buf, logger.INFO,
		logger.WithMiddleware(addRegion, dropHealth),
	)

	log.Info("health check", nil)
	log.Info("real request", nil)

	output := buf.String()
	fmt.Println("health check dropped:", !strings.Contains(output, "health check"))
	fmt.Println("region added:", strings.Contains(output, "us-east-1"))
	// Output:
	// health check dropped: true
	// region added: true
}

func ExampleWithFallbackWriter() {
	// Primary writer and fallback — if primary fails, fallback gets the entry
	var fallback bytes.Buffer
	log := logger.New(os.Stdout, logger.INFO,
		logger.WithFallbackWriter(&fallback),
	)
	_ = log // use normally; if stdout fails, entries go to fallback
}

func ExampleWithStackTrace() {
	var buf bytes.Buffer
	log := logger.New(&buf, logger.INFO,
		logger.WithStackTrace(),
	)

	log.Error("something broke", nil)

	output := buf.String()
	fmt.Println("has stacktrace:", strings.Contains(output, "stacktrace"))
	// Output: has stacktrace: true
}
