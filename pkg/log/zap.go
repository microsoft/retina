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
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var l *ZapLogger

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
	return l
}

type ZapLogger struct {
	*zap.Logger
	lvl         zapcore.Level
	opts        *LogOpts
	closeAISink func()
}

func toLevel(lvl string) zapcore.Level {
	switch lvl {
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "panic":
		return zapcore.PanicLevel
	case "fatal":
		return zapcore.FatalLevel
	case "debug":
		return zapcore.DebugLevel
	default:
		panic("Log level not supported")
	}
}

func EncoderConfig() zapcore.EncoderConfig {
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	return encoderCfg
}

func SetupZapLogger(lOpts *LogOpts) {
	if l != nil {
		return
	}

	lOpts.validate()
	// Setup logger level.
	atom := zap.NewAtomicLevel()
	atom.SetLevel(toLevel(lOpts.Level))
	encoderCfg := EncoderConfig()
	var core, logFmtCore, fileCore, aiCore zapcore.Core

	// Setup a default stdout logger
	logFmtCore = zapcore.NewCore(
		logfmt.NewEncoder(encoderCfg),
		zapcore.Lock(os.Stdout),
		atom,
	)
	l = &ZapLogger{
		Logger: zap.New(logFmtCore, zap.AddCaller()),
		lvl:    atom.Level(),
		opts:   lOpts,
	}
	l.Logger.Debug("Stdout logger enabled")

	if lOpts.ApplicationInsightsID != "" && lOpts.EnableTelemetry {
		// build the AI config
		aiTelemetryConfig := appinsights.NewTelemetryConfiguration(lOpts.ApplicationInsightsID)
		sinkcfg := zapai.SinkConfig{
			GracePeriod:            30 * time.Second, //nolint:gomnd // ignore
			TelemetryConfiguration: *aiTelemetryConfig,
		}
		sinkcfg.MaxBatchSize = 10000
		sinkcfg.MaxBatchInterval = 10 * time.Second //nolint:gomnd // ignore

		// open the AI zap sink
		aisink, aiclose, err := zap.Open(sinkcfg.URI()) //nolint:govet // intentional shadow
		if err != nil {
			l.Logger.Error("failed to open AI sink", zap.Error(err))
			return
		}
		// set the AI sink closer
		l.closeAISink = aiclose
		// build the AI core
		aiCore = zapai.NewCore(toLevel(lOpts.Level), aisink).WithFieldMappers(zapai.DefaultMappers)
		l.Logger.Debug("Application Insights logger enabled")
	}

	if lOpts.File {
		// Setup a file logger if it is enabled
		// lumberjack is Zap endorsed logger rotation library
		fw := zapcore.AddSync(&lumberjack.Logger{
			Filename:   lOpts.FileName,
			MaxSize:    lOpts.MaxFileSizeMB, // megabytes
			MaxBackups: l.opts.MaxBackups,
			MaxAge:     l.opts.MaxAgeDays, // days
		})
		fileCore = zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderCfg),
			fw,
			atom,
		)
		l.Logger.Debug("File logger enabled")
	}
	// Create a new teecore including all the enabled zap cores
	if fileCore != nil && aiCore != nil {
		core = zapcore.NewTee(logFmtCore, fileCore, aiCore)
	} else if fileCore != nil {
		core = zapcore.NewTee(logFmtCore, fileCore)
	} else if aiCore != nil {
		core = zapcore.NewTee(logFmtCore, aiCore)
	} else {
		core = zapcore.NewTee(logFmtCore)
	}
	l.Logger = zap.New(core, zap.AddCaller())
	l.Logger = l.Logger.With(
		zap.String("goversion", runtime.Version()),
		zap.String("os", runtime.GOOS),
		zap.String("arch", runtime.GOARCH),
		zap.Int("numcores", runtime.NumCPU()),
		zap.String("hostname", os.Getenv("HOSTNAME")),
		zap.String("podname", os.Getenv("POD_NAME")),
	)
}

func (l *ZapLogger) SetLevel(lvl string) {
	l.lvl = toLevel(lvl)
	_ = l.Logger.Sync()
}

func (l *ZapLogger) Level() int {
	return int(l.lvl)
}

func (l *ZapLogger) Close() {
	if l.closeAISink != nil {
		l.closeAISink()
	}
	_ = l.Logger.Sync()
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

// AddFields adds custom fields to the logger
func (l *ZapLogger) AddFields(fields ...zap.Field) {
	l.Logger = l.Logger.With(fields...)
}
