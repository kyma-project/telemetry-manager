package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
)

const (
	serverPort = 8080
)

var (
	logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

	hdErrorsMeter  metric.Int64Counter
	cpuEnergyMeter metric.Float64Histogram

	hdErrorsAttribute  = attribute.String("device", "/dev/sda")
	cpuEnergyAttribute = attribute.String("core", "0")

	tracer = otel.Tracer("sample-app")
	meter  = otel.Meter("sample-app")

	terminateEndpoint string
)

func init() {
	var err error
	if _, err = meter.Float64ObservableGauge(
		"cpu.temperature.celsius",
		metric.WithDescription("Current temperature of the CPU."),
		metric.WithUnit("celsius"),
		metric.WithFloat64Callback(func(_ context.Context, o metric.Float64Observer) error {
			o.Observe(randomTemp())
			return nil
		}),
	); err != nil {
		panic(err)
	}

	hdErrorsMeter, err = meter.Int64Counter(
		"hd.errors.total",
		metric.WithDescription("Number of hard-disk errors."),
		metric.WithUnit("{device}"),
	)
	if err != nil {
		panic(err)
	}

	cpuEnergyMeter, err = meter.Float64Histogram(
		"cpu.energy.watt",
		metric.WithDescription("Current power usage reported by the CPU."),
		metric.WithUnit("core"),
	)
	if err != nil {
		panic(err)
	}

	var ok bool
	if terminateEndpoint, ok = os.LookupEnv("TERMINATE_ENDPOINT"); !ok {
		terminateEndpoint = fmt.Sprintf("localhost:%d", serverPort)
	}
	logger.Info("Using terminate endpoint: " + terminateEndpoint)
}

// randomTemp generates the temperature ranging from 60 to 90
func randomTemp() float64 {
	return math.Round(rand.Float64()*300)/10 + 60
}

// randomEnergy generates the energy ranging from 0 to 100
func randomEnergy() float64 {
	return math.Round(rand.Float64() * 100)
}

func randBool() bool {
	return rand.Intn(2) == 1
}

func forwardHandler(w http.ResponseWriter, r *http.Request) {

	_, span := tracer.Start(r.Context(), "forward")
	defer span.End()

	requestURL := fmt.Sprintf("http://%s/terminate", terminateEndpoint)
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		logger.ErrorContext(r.Context(), "client: could not create request", slog.String("error", err.Error()), slog.String("traceId", span.SpanContext().TraceID().String()))

		span.RecordError(err)
		span.SetStatus(codes.Error, "error")

		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error")

		return
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.ErrorContext(r.Context(), "client: error making http request", slog.String("error", err.Error()), slog.String("traceId", span.SpanContext().TraceID().String()))

		span.RecordError(err)
		span.SetStatus(codes.Error, "error")

		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error")

		return
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		logger.ErrorContext(r.Context(), "client: could not read response body", slog.String("error", err.Error()), slog.String("traceId", span.SpanContext().TraceID().String()))

		span.RecordError(err)
		span.SetStatus(codes.Error, "error")

		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error")

		return
	}

	logger.InfoContext(r.Context(), "Forwarded successful", slog.String("status", res.Status), slog.String("traceId", span.SpanContext().TraceID().String()))
	w.WriteHeader(res.StatusCode)
	fmt.Fprint(w, string(resBody))
}

func terminateHandler(w http.ResponseWriter, r *http.Request) {

	_, span := tracer.Start(r.Context(), "terminate")
	defer span.End()

	span.SetAttributes(hdErrorsAttribute, cpuEnergyAttribute)
	cpuEnergyMeter.Record(r.Context(), randomEnergy(), metric.WithAttributes(cpuEnergyAttribute))

	if randBool() {
		span.RecordError(fmt.Errorf("error"))
		span.SetStatus(codes.Error, "error")

		hdErrorsMeter.Add(r.Context(), 5, metric.WithAttributes(hdErrorsAttribute))

		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error")

		return
	}
	logger.InfoContext(r.Context(), "Terminated successful", slog.String("traceId", span.SpanContext().TraceID().String()))

	hdErrorsMeter.Add(r.Context(), 1, metric.WithAttributes(hdErrorsAttribute))
	fmt.Fprintf(w, "Success")
}

func main() {
	ctx := context.Background()

	// Instantiate the trace and metric providers
	tp := newTraceProvider(newTraceExporter(ctx))
	mp := newMeterProvider(newMetricExporter(ctx))

	// Handle shutdown properly so nothing leaks.
	defer func() {
		_ = tp.Shutdown(ctx)
		_ = mp.Shutdown(ctx)
	}()

	// Register the trace and metric providers
	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)

	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Configure the HTTP server
	http.DefaultClient.Timeout = 30 * time.Second

	// Wrap the handler with OpenTelemetry instrumentation
	wrappedForwardHandler := otelhttp.NewHandler(http.HandlerFunc(forwardHandler), "auto-forward")
	wrappedTerminateHandler := otelhttp.NewHandler(http.HandlerFunc(terminateHandler), "auto-terminate")

	// Register the handlers
	http.Handle("/forward", wrappedForwardHandler)
	http.Handle("/", wrappedForwardHandler)
	http.Handle("/terminate", wrappedTerminateHandler)

	//Start the HTTP server
	logger.Info("Starting server on port " + strconv.Itoa(serverPort))

	err := http.ListenAndServe(fmt.Sprintf(":%d", serverPort), nil)
	if err != nil {
		panic(err)
	}
}
