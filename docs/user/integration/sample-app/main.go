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

	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	hdErrorsAttributeSda = attribute.String("device", "/dev/sda")
	hdErrorsAttributeSdb = attribute.String("device", "/dev/sdb")
	cpuEnergyAttribute0  = attribute.String("core", "0")
	cpuEnergyAttribute1  = attribute.String("core", "1")

	tracer = otel.Tracer("")
	meter  = otel.Meter("")

	terminateEndpoint string
)

// init registers the metrics and sets up the environment
func initMetrics() error {
	var err error

	hdErrorsMeter, err = meter.Int64Counter(
		"hd.errors.total",
		metric.WithDescription("Number of hard-disk errors."),
		metric.WithUnit("{device}"),
	)
	if err != nil {
		return fmt.Errorf("error creating hd.errors.total counter: %w", err)
	}

	cpuEnergyMeter, err = meter.Float64Histogram(
		"cpu.energy.watt",
		metric.WithDescription("Current power usage reported by the CPU."),
		metric.WithUnit("core"),
	)
	if err != nil {
		return fmt.Errorf("error creating cpu.energy.watt histogram: %w", err)
	}

	if _, err = meter.Float64ObservableGauge(
		"cpu.temperature.celsius",
		metric.WithDescription("Current temperature of the CPU."),
		metric.WithUnit("celsius"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			o.Observe(randomTemp())
			//as there is no async histogram, use this callbacl to also record the other meters
			hdErrorsMeter.Add(ctx, 1, metric.WithAttributes(hdErrorsAttributeSdb))
			cpuEnergyMeter.Record(ctx, randomEnergy(), metric.WithAttributes(cpuEnergyAttribute1))
			return nil
		}),
	); err != nil {
		return fmt.Errorf("error creating cpu.temperature.celsius gauge: %w", err)
	}
	return nil
}

func initTerminateEndpoint() {
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

// randBool generates a random bool
func randBool() bool {
	return rand.Intn(2) == 1
}

// forwardHandler handles the incoming request by forwarding it to the terminate endpoint
func forwardHandler(w http.ResponseWriter, r *http.Request) {

	// Initialize a new span for the current trace
	ctx, span := tracer.Start(r.Context(), "forward")
	defer span.End()

	// Forward the request to the terminate endpoint using the instrumented HTTP client, so that trace context gets propagated
	requestURL := fmt.Sprintf("http://%s/terminate", terminateEndpoint)
	res, err := otelhttp.Get(ctx, requestURL)
	if err != nil {
		logger.ErrorContext(ctx, "client: error making http request", slog.String("error", err.Error()), slog.String("traceId", span.SpanContext().TraceID().String()))

		span.RecordError(err)
		span.SetStatus(codes.Error, "exception in client call")

		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error")

		return
	}
	defer res.Body.Close()
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		logger.ErrorContext(ctx, "client: could not read response body", slog.String("error", err.Error()), slog.String("traceId", span.SpanContext().TraceID().String()))

		span.RecordError(err)
		span.SetStatus(codes.Error, "client call returned malformed body")

		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error")

		return
	}

	if res.StatusCode/100 != 2 {
		logger.ErrorContext(ctx, "client: received error response code", slog.String("status", res.Status), slog.String("traceId", span.SpanContext().TraceID().String()))
		span.SetStatus(codes.Error, "error")
	} else {
		logger.InfoContext(ctx, "Forwarded successful", slog.String("status", res.Status), slog.String("traceId", span.SpanContext().TraceID().String()))
	}

	w.WriteHeader(res.StatusCode)
	fmt.Fprint(w, string(resBody))
}

// terminateHandler handles the incoming request by terminating it randomly with a success or error response
func terminateHandler(w http.ResponseWriter, r *http.Request) {

	// Initialize a new span for the current trace with enriched attributes
	ctx, span := tracer.Start(r.Context(), "terminate")
	defer span.End()
	span.SetAttributes(hdErrorsAttributeSda, cpuEnergyAttribute0)

	// Record metric and enrich it with attributes
	cpuEnergyMeter.Record(ctx, randomEnergy(), metric.WithAttributes(cpuEnergyAttribute0))

	// Terminate the request randomly with a success or error response
	if randBool() {
		span.RecordError(fmt.Errorf("random logic decided to fail the request"))
		span.SetStatus(codes.Error, "error")

		hdErrorsMeter.Add(ctx, 5, metric.WithAttributes(hdErrorsAttributeSda))

		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error")

		logger.ErrorContext(ctx, "Terminated errorful", slog.String("traceId", span.SpanContext().TraceID().String()))

		return
	}
	logger.InfoContext(ctx, "Terminated successful", slog.String("traceId", span.SpanContext().TraceID().String()))

	hdErrorsMeter.Add(ctx, 1, metric.WithAttributes(hdErrorsAttributeSda))
	fmt.Fprintf(w, "Success")
}

func run() error {
	ctx := context.Background()

	// Instantiate the trace and metric providers
	res, err := newOtelResource()
	if err != nil {
		return fmt.Errorf("creating resource: %w", err)
	}

	te, err := newTraceExporter(ctx)
	if err != nil {
		return fmt.Errorf("creating trace exporter: %w", err)
	}
	tp := newTraceProvider(te, res)

	mr, err := newMetricReader(ctx)
	if err != nil {
		return fmt.Errorf("creating meter provider: %w", err)
	}
	mp := newMeterProvider(mr, res)

	// Handle shutdown properly so nothing leaks.
	defer func() {
		_ = tp.Shutdown(ctx)
		_ = mp.Shutdown(ctx)
	}()

	// Register the trace and metric providers
	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)

	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Initialize the metrics
	err = initMetrics()
	if err != nil {
		return fmt.Errorf("error initializing metrics: %w", err)
	}

	// Configure the HTTP server
	http.DefaultClient.Timeout = 30 * time.Second

	// Initialize the forward handler with the terminate endpoint to use
	initTerminateEndpoint()

	// Wrap the handler with OpenTelemetry instrumentation
	wrappedForwardHandler := otelhttp.NewHandler(http.HandlerFunc(forwardHandler), "auto-forward")
	wrappedTerminateHandler := otelhttp.NewHandler(http.HandlerFunc(terminateHandler), "auto-terminate")

	// Register the handlers
	http.Handle("/forward", wrappedForwardHandler)
	http.Handle("/terminate", wrappedTerminateHandler)
	http.Handle("/", wrappedForwardHandler)

	// Register metrics endpoint in case a prometheus exporter is used
	http.Handle("/metrics", otelhttp.NewHandler(promhttp.Handler(), "metrics"))

	//Start the HTTP server
	logger.Info("Starting server on port " + strconv.Itoa(serverPort))
	err = http.ListenAndServe(fmt.Sprintf(":%d", serverPort), nil)
	if err != nil {
		return fmt.Errorf("error starting server: %w", err)
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		logger.Error("Error running server", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
