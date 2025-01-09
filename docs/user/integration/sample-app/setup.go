package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
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
	if os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") != "" {
		exporter, err := otlptracegrpc.New(ctx)
		if err != nil {
			panic(fmt.Errorf("creating OTLP trace exporter: %w", err))
		}
		return exporter
	}
	exporter, err := stdouttrace.New()
	if err != nil {
		panic(fmt.Errorf("creating stdout trace exporter: %w", err))
	}
	return exporter
}

func newMeterProvider(exp metric.Exporter) *metric.MeterProvider {
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
		metric.WithReader(metric.NewPeriodicReader(exp,
			// Default is 1m. Set to 10s for demonstrative purposes.
			metric.WithInterval(10*time.Second))),
	)
	return meterProvider
}

func newMetricExporter(ctx context.Context) metric.Exporter {
	if os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT") != "" {
		exporter, err := otlpmetricgrpc.New(ctx)
		if err != nil {
			panic(fmt.Errorf("creating OTLP metric exporter: %w", err))
		}
		return exporter
	}
	exporter, err := stdoutmetric.New()
	if err != nil {
		panic(fmt.Errorf("creating stdout metric exporter: %w", err))
	}
	return exporter
}
