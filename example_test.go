package logger_test

import (
	"context"
	"fmt"
	"os"

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
