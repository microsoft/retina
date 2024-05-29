// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package log

import (
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/Azure/azure-container-networking/zapai"
	"github.com/go-chi/chi/middleware"
	logfmt "github.com/jsternberg/zap-logfmt"
	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var global *ZapLogger

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
	return global
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
	if global != nil {
		return global, nil
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
	global = logger
	return global, nil
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
