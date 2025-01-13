package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func newTraceProvider(exp trace.SpanExporter) *trace.TracerProvider {
	// Ensure default SDK resources and the required service name are set.
	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("sample-app"),
		),
	)

	if err != nil {
		panic(fmt.Errorf("creating resource: %w", err))
	}

	return trace.NewTracerProvider(
		trace.WithBatcher(exp),
		trace.WithResource(r),
	)
}

func newTraceExporter(ctx context.Context) trace.SpanExporter {
	var exporterEnv = os.Getenv("OTEL_TRACES_EXPORTER")
	var endpointEnv = os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")

	if exporterEnv == "otlp" || endpointEnv != "" {
		exporter, err := otlptracegrpc.New(ctx)
		if err != nil {
			panic(fmt.Errorf("creating OTLP trace exporter: %w", err))
		}
		logger.Info("using OTLP trace exporter with endpoint: " + exporterEnv)
		return exporter
	}
	exporter, err := stdouttrace.New()
	if err != nil {
		panic(fmt.Errorf("creating stdout trace exporter: %w", err))
	}
	logger.Info("using console trace exporter")
	return exporter
}

func newMeterProvider(exp metric.Reader) *metric.MeterProvider {
	// Ensure default SDK resources and the required service name are set.
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("sample-app"),
		),
	)

	if err != nil {
		panic(fmt.Errorf("creating resource: %w", err))
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(exp),
	)
	return meterProvider
}

func newMetricExporter(ctx context.Context) metric.Reader {
	var exporterEnv = os.Getenv("OTEL_METRICS_EXPORTER")
	var endpointEnv = os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT")

	if exporterEnv == "prometheus" {
		exporter, err := prometheus.New()
		if err != nil {
			panic(fmt.Errorf("creating prometheus metric exporter: %w", err))
		}
		logger.Info("Using Prometheus metric exporter")
		return exporter
	}

	if exporterEnv == "otlp" || endpointEnv != "" {
		exporter, err := otlpmetricgrpc.New(ctx)
		if err != nil {
			panic(fmt.Errorf("creating OTLP metric exporter: %w", err))
		}
		logger.Info("using OTLP metric exporter with endpoint: " + endpointEnv)
		return metric.NewPeriodicReader(exporter, metric.WithInterval(10*time.Second))
	}
	exporter, err := stdoutmetric.New()
	if err != nil {
		panic(fmt.Errorf("creating stdout metric exporter: %w", err))
	}
	logger.Info("using console metric exporter")
	return metric.NewPeriodicReader(exporter,
		// Default is 1m. Set to 10s for demonstrative purposes.
		metric.WithInterval(5*time.Second))
}
