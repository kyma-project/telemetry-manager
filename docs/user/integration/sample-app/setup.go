package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/slogr"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func newOTelSDKLogger() (*logr.Logger, error) {
	sdkLogLevelEnv := os.Getenv("OTEL_LOG_LEVEL")
	if sdkLogLevelEnv == "" {
		sdkLogLevelEnv = "INFO"
	}

	sdkLogLevels := map[string]slog.Level{
		"DEBUG": slog.LevelDebug,
		"INFO":  slog.LevelInfo,
		"WARN":  slog.LevelWarn,
		"ERROR": slog.LevelError,
	}

	slogLogLevel, ok := sdkLogLevels[sdkLogLevelEnv]
	if !ok {
		return nil, fmt.Errorf("invalid log level: %s", sdkLogLevelEnv)
	}

	jsonHandler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slogLogLevel})
	logger := slogr.NewLogr(jsonHandler)
	return &logger, nil
}

func newOtelResource() (*resource.Resource, error) {
	// Ensure default SDK resources and the required service name are set.
	res, err := resource.New(
		context.Background(),
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithAttributes(semconv.ServiceName("sample-app")), // Default service name which might get overriden by OTEL_SERVICE_NAME.
		resource.WithFromEnv(),                                     // Discover and provide attributes from OTEL_RESOURCE_ATTRIBUTES and OTEL_SERVICE_NAME environment variables.
		resource.WithTelemetrySDK(),                                // Discover and provide information about the OpenTelemetry SDK used.
	)

	if err != nil {
		return nil, fmt.Errorf("creating resource: %w", err)
	}
	return res, nil
}

func newTraceProvider(exp trace.SpanExporter, res *resource.Resource) *trace.TracerProvider {
	return trace.NewTracerProvider(
		trace.WithBatcher(exp),
		trace.WithResource(res),
	)
}

func newTraceExporter(ctx context.Context) (trace.SpanExporter, error) {
	exporterEnv := os.Getenv("OTEL_TRACES_EXPORTER")
	endpointEnv := os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")

	if exporterEnv == "otlp" || endpointEnv != "" {
		return newOTLPTraceExporter(ctx)
	}

	// Default to stdout exporter if no OTLP configuration is found
	exporter, err := stdouttrace.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout trace exporter: %w", err)
	}
	logger.Info("Using console trace exporter")
	return exporter, nil
}

func newOTLPTraceExporter(ctx context.Context) (trace.SpanExporter, error) {
	protocol := resolveOTLPProtocol()
	switch protocol {
	case "http/protobuf":
		exporter, err := otlptracehttp.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP OTLP metric exporter: %w", err)
		}
		logger.Info("Using HTTP OTLP metric exporter")
		return exporter, nil
	case "grpc":
		exporter, err := otlptracegrpc.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create gRPC OTLP metric exporter: %w", err)
		}
		return exporter, nil
	default:
		return nil, fmt.Errorf("unsupported OTLP protocol: %s", protocol)
	}
}

func newMeterProvider(exp metric.Reader, res *resource.Resource) *metric.MeterProvider {
	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(exp),
	)
	return meterProvider
}

func newMetricReader(ctx context.Context) (metric.Reader, error) {
	exporterEnv := os.Getenv("OTEL_METRICS_EXPORTER")
	endpointEnv := os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT")

	if exporterEnv == "prometheus" {
		reader, err := prometheus.New()
		if err != nil {
			return nil, fmt.Errorf("creating prometheus metric reader: %w", err)
		}
		logger.Info("Using Prometheus metric exporter")
		return reader, nil
	}

	if exporterEnv == "otlp" || endpointEnv != "" {
		otlpExporter, err := newOTLPMetricExporter(ctx)
		if err != nil {
			return nil, err
		}

		return metric.NewPeriodicReader(otlpExporter, metric.WithInterval(10*time.Second)), nil
	}

	exporter, err := stdoutmetric.New()
	if err != nil {
		return nil, fmt.Errorf("creating stdout metric exporter: %w", err)
	}
	logger.Info("Using console metric exporter")
	return metric.NewPeriodicReader(exporter,
		// Default is 1m. Set to 10s for demonstrative purposes.
		metric.WithInterval(5*time.Second)), nil
}

func newOTLPMetricExporter(ctx context.Context) (metric.Exporter, error) {
	protocol := resolveOTLPProtocol()
	switch protocol {
	case "http/protobuf":
		exporter, err := otlpmetrichttp.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP OTLP metric exporter: %w", err)
		}
		logger.Info("Using HTTP OTLP metric exporter")
		return exporter, nil
	case "grpc":
		exporter, err := otlpmetricgrpc.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create gRPC OTLP metric exporter: %w", err)
		}
		return exporter, nil
	default:
		return nil, fmt.Errorf("unsupported OTLP protocol: %s", protocol)
	}
}

func resolveOTLPProtocol() string {
	protocolEnv := os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")
	if protocolEnv == "" {
		return "grpc"
	}
	return protocolEnv
}
