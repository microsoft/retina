// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package log

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Azure/azure-container-networking/zapai"
	"github.com/go-chi/chi/middleware"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	logfmt "github.com/jsternberg/zap-logfmt"
	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// global holds the configured ZapLogger. Written once by SetupZapLogger and
// read by every helper in this package; atomic for race-free access from
// goroutines started after (or concurrently with) SetupZapLogger.
var (
	global   atomic.Pointer[ZapLogger]
	initOnce sync.Once
)

func loadGlobal() *ZapLogger { return global.Load() }

const (
	defaultFileName   = "retina.log"
	defaultMaxSize    = 50 // MB
	defaultMaxAge     = 30 // days
	defaultMaxBackups = 3
)

type LogOpts struct {
	Level                 string
	File                  bool
	FileName              string
	MaxFileSizeMB         int
	MaxBackups            int
	MaxAgeDays            int
	ApplicationInsightsID string
	EnableTelemetry       bool
}

func GetDefaultLogOpts() *LogOpts {
	return &LogOpts{
		Level: "info",
		File:  false,
	}
}

func Logger() *ZapLogger {
	initOnce.Do(func() {
		if loadGlobal() == nil {
			_, _ = SetupZapLogger(GetDefaultLogOpts())
		}
	})
	return loadGlobal()
}

type ZapLogger struct {
	*zap.Logger
	lvl    zapcore.Level
	closeF []func()
}

func EncoderConfig() zapcore.EncoderConfig {
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	return encoderCfg
}

func SetupZapLogger(lOpts *LogOpts, persistentFields ...zap.Field) (*ZapLogger, error) {
	if g := loadGlobal(); g != nil {
		return g, nil
	}
	logger := &ZapLogger{}

	lOpts.validate()
	// Setup logger level.
	lev, err := zap.ParseAtomicLevel(lOpts.Level)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse log level")
	}
	encoderCfg := EncoderConfig()

	// Setup a default stdout logger
	core := zapcore.NewCore(
		logfmt.NewEncoder(encoderCfg),
		zapcore.Lock(os.Stdout),
		lev,
	)

	if lOpts.ApplicationInsightsID != "" && lOpts.EnableTelemetry {
		persistentFields = append(persistentFields,
			zap.String("goversion", runtime.Version()),
			zap.String("os", runtime.GOOS),
			zap.String("arch", runtime.GOARCH),
			zap.Int("numcores", runtime.NumCPU()),
			zap.String("hostname", os.Getenv("HOSTNAME")),
			zap.String("podname", os.Getenv("POD_NAME")),
		)
		// build the AI config
		aiTelemetryConfig := appinsights.NewTelemetryConfiguration(lOpts.ApplicationInsightsID)
		sinkcfg := zapai.SinkConfig{
			GracePeriod:            30 * time.Second, //nolint:gomnd // ignore
			TelemetryConfiguration: *aiTelemetryConfig,
		}
		sinkcfg.MaxBatchSize = 10000
		sinkcfg.MaxBatchInterval = 10 * time.Second //nolint:gomnd // ignore

		// open the AI zap aiSink
		aiSink, closeAI, err := zap.Open(sinkcfg.URI()) //nolint:govet // intentional shadow
		if err != nil {
			return nil, errors.Wrap(err, "failed to open AI sink")
		}
		logger.closeF = append(logger.closeF, closeAI)
		// build the AI aicore
		aicore := zapai.NewCore(lev, aiSink).
			WithFieldMappers(zapai.DefaultMappers).
			With(persistentFields)
		core = zapcore.NewTee(core, aicore)
	}

	if lOpts.File {
		l := &lumberjack.Logger{
			Filename:   lOpts.FileName,
			MaxSize:    lOpts.MaxFileSizeMB, // megabytes
			MaxBackups: lOpts.MaxBackups,
			MaxAge:     lOpts.MaxAgeDays, // days
		}
		logger.closeF = append(logger.closeF, func() { l.Close() })
		fw := zapcore.AddSync(l)
		filecore := zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderCfg),
			fw,
			lev,
		)
		core = zapcore.NewTee(core, filecore)
	}
	logger.Logger = zap.New(core, zap.AddCaller())
	global.Store(logger)
	return logger, nil
}

func (l *ZapLogger) Close() {
	_ = l.Logger.Sync()
	for _, close := range l.closeF {
		close()
	}
}

func (l *ZapLogger) Named(name string) *ZapLogger {
	return &ZapLogger{
		Logger: l.Logger.Named(name),
		lvl:    l.lvl,
	}
}

func (l *ZapLogger) GetZappedMiddleware() func(next http.Handler) http.Handler {
	return l.zappedMiddleware
}

func (l *ZapLogger) zappedMiddleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		var requestID string
		if reqID := r.Context().Value(middleware.RequestIDKey); reqID != nil {
			requestID = reqID.(string)
		}
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		latency := time.Since(start)

		if l.Logger != nil {
			fields := []zapcore.Field{
				zap.Int("status", ww.Status()),
				zap.Duration("took", latency),
				zap.Int64("latency", latency.Nanoseconds()),
				zap.String("remote", r.RemoteAddr),
				zap.String("request", r.RequestURI),
				zap.String("method", r.Method),
			}
			if requestID != "" {
				fields = append(fields, zap.String("request-id", requestID))
			}
			l.Logger.Info("request completed", fields...)
		}
	}

	return http.HandlerFunc(fn)
}

func (lOpts *LogOpts) validate() {
	if lOpts.Level == "" {
		lOpts.Level = "info"
	}
	if lOpts.File {
		if lOpts.FileName == "" {
			lOpts.FileName = defaultFileName
		}
		if lOpts.MaxFileSizeMB == 0 {
			lOpts.MaxFileSizeMB = defaultMaxSize
		}
		if lOpts.MaxBackups == 0 {
			lOpts.MaxBackups = defaultMaxBackups
		}
		if lOpts.MaxAgeDays == 0 {
			lOpts.MaxAgeDays = defaultMaxAge
		}
	}
}

// SlogHandler returns an slog.Handler backed by the global zap core.
// All slog messages will flow through zap's pipeline (stdout + Application Insights).
// The returned handler resolves the global zap core on every call, so it keeps
// working even if registered (e.g. via logging.AddHandlers) before SetupZapLogger
// runs — once the global is initialized, records start flowing to AI.
func SlogHandler() slog.Handler {
	return &lazyZapHandler{}
}

// lazyZapHandler resolves the Retina zap core at call time. This lets callers
// register a single slog.Handler into Cilium's MultiSlogHandler (or capture
// it via slog.Default/.With) before SetupZapLogger runs — once `global` is
// set, records flow through zap → stdout + Application Insights.
//
// `ops` records WithAttrs / WithGroup calls in their original order so the
// inner zap-backed handler is built with attribute-group nesting identical to
// what the caller requested (per the slog.Handler contract).
type lazyZapHandler struct {
	ops []lazyOp
}

type lazyOp struct {
	attrs []slog.Attr // when non-nil, a WithAttrs op
	group string      // when attrs is nil and group != "", a WithGroup op
}

func (h *lazyZapHandler) inner() slog.Handler {
	g := loadGlobal()
	if g == nil {
		return nil
	}
	var sh slog.Handler = zapslog.NewHandler(g.Core(), zapslog.WithCaller(true))
	for _, op := range h.ops {
		if op.attrs != nil {
			sh = sh.WithAttrs(op.attrs)
		} else {
			sh = sh.WithGroup(op.group)
		}
	}
	return sh
}

func (h *lazyZapHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	sh := h.inner()
	if sh == nil {
		return true // accept records; they'll be dropped below if still uninitialized
	}
	return sh.Enabled(ctx, lvl)
}

func (h *lazyZapHandler) Handle(ctx context.Context, r slog.Record) error {
	sh := h.inner()
	if sh == nil {
		return nil // zap not ready yet; Cilium's own text handler still logs to stderr
	}
	//nolint:wrapcheck // passthrough to the inner slog.Handler; wrapping would mangle zap diagnostics
	return sh.Handle(ctx, r)
}

func (h *lazyZapHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	ops := make([]lazyOp, len(h.ops), len(h.ops)+1)
	copy(ops, h.ops)
	ops = append(ops, lazyOp{attrs: attrs})
	return &lazyZapHandler{ops: ops}
}

func (h *lazyZapHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h // per slog.Handler contract
	}
	ops := make([]lazyOp, len(h.ops), len(h.ops)+1)
	copy(ops, h.ops)
	ops = append(ops, lazyOp{group: name})
	return &lazyZapHandler{ops: ops}
}

// SetDefaultSlog sets Go's global slog default to use the zap-backed handler.
// After calling this, slog.Default() returns a logger that routes through zap.
func SetDefaultSlog() {
	slog.SetDefault(slog.New(SlogHandler()))
}

// SlogLogger returns a new *slog.Logger backed by the global zap core.
func SlogLogger() *slog.Logger {
	return slog.New(SlogHandler())
}

// LogrLogger returns a logr.Logger backed by the global zap logger.
// This is useful for integrating with controller-runtime and other libraries
// that use logr.Logger, ensuring consistent log format across the application.
//
// The returned logr.Logger is backed by a logr.LogSink that resolves the zap
// core on every call, so controller-runtime loggers set up before
// SetupZapLogger runs automatically start flowing to Application Insights
// once the zap global is initialized.
func LogrLogger() logr.Logger {
	return logr.New(&lazyLogrSink{})
}

// lazyLogrSink resolves the Retina zap.Logger on every call and delegates to
// a fresh zapr sink. A production-zap fallback keeps the sink safe to use
// before SetupZapLogger runs.
type lazyLogrSink struct {
	name   string
	values []any
}

func (s *lazyLogrSink) delegate() logr.LogSink {
	g := loadGlobal()
	var zl *zap.Logger
	if g != nil {
		zl = g.Logger
	} else {
		zl = zap.Must(zap.NewProduction())
	}
	lr := zapr.NewLogger(zl)
	if s.name != "" {
		lr = lr.WithName(s.name)
	}
	if len(s.values) > 0 {
		lr = lr.WithValues(s.values...)
	}
	return lr.GetSink()
}

func (s *lazyLogrSink) Init(ri logr.RuntimeInfo) { s.delegate().Init(ri) }
func (s *lazyLogrSink) Enabled(level int) bool   { return s.delegate().Enabled(level) }
func (s *lazyLogrSink) Info(level int, msg string, kv ...any) {
	s.delegate().Info(level, msg, kv...)
}

func (s *lazyLogrSink) Error(err error, msg string, kv ...any) {
	s.delegate().Error(err, msg, kv...)
}

func (s *lazyLogrSink) WithValues(kv ...any) logr.LogSink {
	nv := make([]any, 0, len(s.values)+len(kv))
	nv = append(nv, s.values...)
	nv = append(nv, kv...)
	return &lazyLogrSink{name: s.name, values: nv}
}

func (s *lazyLogrSink) WithName(name string) logr.LogSink {
	out := s.name
	if out == "" {
		out = name
	} else {
		out = out + "." + name
	}
	return &lazyLogrSink{name: out, values: s.values}
}
