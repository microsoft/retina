// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package exporter

// import (
// 	"context"
// 	"time"

// 	"github.com/microsoft/retina/pkg/config"
// 	"github.com/microsoft/retina/pkg/log"
// 	"github.com/microsoft/retina/pkg/plugin/api"
// 	"go.opentelemetry.io/otel"
// 	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric"
// 	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
// 	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
// 	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
// 	"go.opentelemetry.io/otel/metric/global"
// 	"go.opentelemetry.io/otel/propagation"
// 	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
// 	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
// 	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
// 	"go.opentelemetry.io/otel/sdk/resource"
// 	sdktrace "go.opentelemetry.io/otel/sdk/trace"
// 	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
// 	"go.uber.org/zap"
// 	"google.golang.org/grpc"
// )

// type OtelAgent struct {
// 	l                    *log.ZapLogger
// 	agentAddress         string
// 	collectPeriodSeconds time.Duration
// }

// func NewOtelAgent(l *log.ZapLogger, c *config.Config) *OtelAgent {
// 	return &OtelAgent{
// 		l:                    l,
// 		agentAddress:         c.OtelAgent.AgentAddress,
// 		collectPeriodSeconds: time.Duration(c.OtelAgent.CollectPeriodSeconds) * time.Second,
// 	}
// }

// func (o *OtelAgent) Start(ctx context.Context) func() {
// 	metricPusher := setUpMetricPusher(ctx, o.agentAddress, o.collectPeriodSeconds, o.l)
// 	traceExp := setUpTracerExporter(ctx, o.agentAddress, o.l)

// 	return func() {
// 		cxt, cancel := context.WithTimeout(ctx, time.Second)
// 		defer cancel()
// 		// pushes any last exports to the receiver
// 		if err := metricPusher.Stop(cxt); err != nil {
// 			otel.Handle(err)
// 		}
// 		// shutdown will flush any remaining spans and shutdown the exporter
// 		if err := traceExp.Shutdown(cxt); err != nil {
// 			otel.Handle(err)
// 		}
// 	}
// }

// func setUpTracerExporter(ctx context.Context, addr string, l *log.ZapLogger) *otlptrace.Exporter {
// 	traceClient := otlptracegrpc.NewClient(
// 		otlptracegrpc.WithInsecure(),
// 		otlptracegrpc.WithEndpoint(addr),
// 		otlptracegrpc.WithDialOption(grpc.WithBlock()),
// 	)
// 	traceExp, err := otlptrace.New(ctx, traceClient)
// 	handleError(err, "Failed to create the collector trace exporter", l)
// 	res, err := resource.New(ctx,
// 		resource.WithFromEnv(),
// 		resource.WithProcess(),
// 		resource.WithTelemetrySDK(),
// 		resource.WithHost(),
// 		resource.WithAttributes(
// 			// the service name used to display traces in backends
// 			semconv.ServiceNameKey.String(api.ServiceName),
// 		),
// 	)
// 	handleError(err, "Failed to create tracer resource", l)
// 	// Register the trace exporter with a TracerProvider, using a batch
// 	// span processor to aggregate spans before export.
// 	bsp := sdktrace.NewBatchSpanProcessor(traceExp)
// 	tracerProvider := sdktrace.NewTracerProvider(
// 		sdktrace.WithSampler(sdktrace.AlwaysSample()),
// 		sdktrace.WithResource(res),
// 		sdktrace.WithSpanProcessor(bsp),
// 	)
// 	// set global propagator to tracecontext (the default is no-op).
// 	otel.SetTextMapPropagator(propagation.TraceContext{})
// 	otel.SetTracerProvider(tracerProvider)
// 	return traceExp
// }

// func setUpMetricPusher(ctx context.Context, addr string, collectPeriod time.Duration, l *log.ZapLogger) *controller.Controller {
// 	metricClient := otlpmetricgrpc.NewClient(
// 		otlpmetricgrpc.WithInsecure(),
// 		otlpmetricgrpc.WithEndpoint(addr))
// 	metricExp, err := otlpmetric.New(ctx, metricClient)
// 	handleError(err, "Failed to create the collector metric exporter", l)

// 	pusher := controller.New(
// 		processor.NewFactory(
// 			simple.NewWithHistogramDistribution(),
// 			metricExp,
// 		),
// 		controller.WithExporter(metricExp),
// 		controller.WithCollectPeriod(collectPeriod),
// 	)
// 	global.SetMeterProvider(pusher)
// 	err = pusher.Start(ctx)
// 	handleError(err, "Failed to start otel meter pusher", l)
// 	return pusher
// }

// func handleError(err error, message string, l *log.ZapLogger) {
// 	if err != nil {
// 		l.Error(message, zap.Error(err))
// 	}
// }
