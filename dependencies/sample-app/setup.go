package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/otlptranslator"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
)

const (
	initialInterval = 5 * time.Second
	maxInterval     = 30 * time.Second
	maxElapsedTime  = 5 * time.Minute
	readerInterval  = 10 * time.Second
)

var (
	// custom  retry config with increased MaxElapsedTime, rest are OTel SDK defaults
	retryConfig = struct {
		Enabled         bool
		InitialInterval time.Duration
		MaxInterval     time.Duration
		MaxElapsedTime  time.Duration
	}{
		Enabled:         true,
		InitialInterval: initialInterval,
		MaxInterval:     maxInterval,
		MaxElapsedTime:  maxElapsedTime,
	}
)

func newOTelSDKLogger(ctx context.Context) (*logr.Logger, error) {
	sdkLogLevelEnv := os.Getenv("OTEL_LOG_LEVEL")
	if sdkLogLevelEnv == "" {
		sdkLogLevelEnv = "INFO"
	} else {
		sdkLogLevelEnv = strings.ToUpper(sdkLogLevelEnv)
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

	logger.InfoContext(ctx, "Using slog logger for OTel SDK", "level", slogLogLevel)

	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slogLogLevel})
	logger := logr.FromSlogHandler(jsonHandler)

	return &logger, nil
}

func newOtelResource() (*resource.Resource, error) {
	// Ensure default SDK resources and the required service name are set.
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(attribute.String("service.name", "sample-app")), // Default service name which might get overridden by OTEL_SERVICE_NAME.
		resource.WithFromEnv(),      // Discover and provide attributes from OTEL_RESOURCE_ATTRIBUTES and OTEL_SERVICE_NAME environment variables.
		resource.WithTelemetrySDK(), // Discover and provide information about the OpenTelemetry SDK used.
	)
	if err != nil {
		return nil, fmt.Errorf("creating resource: %w", err)
	}

	return res, nil
}

func newTraceProvider(exp tracesdk.SpanExporter, res *resource.Resource) *tracesdk.TracerProvider {
	return tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exp),
		tracesdk.WithResource(res),
	)
}

func newTraceExporter(ctx context.Context) (tracesdk.SpanExporter, error) {
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

	logger.InfoContext(ctx, "Using console trace exporter")

	return exporter, nil
}

//nolint:dupl // no duplicate code, this is a separate function for OTLP trace exporter
func newOTLPTraceExporter(ctx context.Context) (tracesdk.SpanExporter, error) {
	protocol := resolveOTLPProtocol()
	switch protocol {
	case "http/protobuf":
		exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithRetry(otlptracehttp.RetryConfig(retryConfig)))
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP OTLP metric exporter: %w", err)
		}

		logger.InfoContext(ctx, "Using HTTP OTLP trace exporter")

		return exporter, nil
	case "grpc":
		exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithRetry(otlptracegrpc.RetryConfig(retryConfig)))
		if err != nil {
			return nil, fmt.Errorf("failed to create gRPC OTLP metric exporter: %w", err)
		}

		logger.InfoContext(ctx, "Using HTTP gRPC trace exporter")

		return exporter, nil
	default:
		return nil, fmt.Errorf("unsupported OTLP protocol: %s", protocol)
	}
}

func newMeterProvider(exp metricsdk.Reader, res *resource.Resource) *metricsdk.MeterProvider {
	meterProvider := metricsdk.NewMeterProvider(
		metricsdk.WithResource(res),
		metricsdk.WithReader(exp),
	)

	return meterProvider
}

func newMetricReader(ctx context.Context) (metricsdk.Reader, error) {
	exporterEnv := os.Getenv("OTEL_METRICS_EXPORTER")
	endpointEnv := os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT")

	if exporterEnv == "prometheus" {
		reader, err := prometheus.New(prometheus.WithTranslationStrategy(otlptranslator.NoUTF8EscapingWithSuffixes))
		if err != nil {
			return nil, fmt.Errorf("creating prometheus metric reader: %w", err)
		}

		logger.InfoContext(ctx, "Using Prometheus metric exporter")

		return reader, nil
	}

	if exporterEnv == "otlp" || endpointEnv != "" {
		otlpExporter, err := newOTLPMetricExporter(ctx)
		if err != nil {
			return nil, err
		}

		return metricsdk.NewPeriodicReader(otlpExporter, metricsdk.WithInterval(readerInterval)), nil
	}

	exporter, err := stdoutmetric.New()
	if err != nil {
		return nil, fmt.Errorf("creating stdout metric exporter: %w", err)
	}

	logger.InfoContext(ctx, "Using console metric exporter")

	return metricsdk.NewPeriodicReader(exporter,
		// Default is 1m. Set to 10s for demonstrative purposes.
		metricsdk.WithInterval(readerInterval)), nil
}

//nolint:dupl // no duplicate code, this is a separate function for OTLP metric exporter
func newOTLPMetricExporter(ctx context.Context) (metricsdk.Exporter, error) {
	protocol := resolveOTLPProtocol()
	switch protocol {
	case "http/protobuf":
		exporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithRetry(otlpmetrichttp.RetryConfig(retryConfig)))
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP OTLP metric exporter: %w", err)
		}

		logger.InfoContext(ctx, "Using HTTP OTLP metric exporter")

		return exporter, nil
	case "grpc":
		exporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithRetry(otlpmetricgrpc.RetryConfig(retryConfig)))
		if err != nil {
			return nil, fmt.Errorf("failed to create gRPC OTLP metric exporter: %w", err)
		}

		logger.InfoContext(ctx, "Using HTTP gRPC metric exporter")

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
