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
	"go.opentelemetry.io/otel/trace"
)

const (
	serverPort        = 8080
	clientTimeout     = 30 * time.Second
	serverReadTimeout = 3 * time.Second
)

var (
	logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

	hdErrorsMeter         metric.Int64Counter
	cpuEnergyMeter        metric.Float64Histogram
	requestURLParamsMeter metric.Int64Counter

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
		"hd.errors",
		metric.WithDescription("Number of hard-disk errors."),
		metric.WithUnit("{device}"),
	)
	if err != nil {
		return fmt.Errorf("error creating hd.errors counter: %w", err)
	}

	cpuEnergyMeter, err = meter.Float64Histogram(
		"cpu.energy.watt",
		metric.WithDescription("Current power usage reported by the CPU."),
		metric.WithUnit("core"),
	)
	if err != nil {
		return fmt.Errorf("error creating cpu.energy.watt histogram: %w", err)
	}

	cpuTempGauge, err := meter.Float64ObservableGauge(
		"cpu.temperature.celsius",
		metric.WithDescription("Current temperature of the CPU."),
		metric.WithUnit("celsius"),
	)
	if err != nil {
		return fmt.Errorf("error creating cpu.temperature.celsius gauge: %w", err)
	}

	_, err = meter.RegisterCallback(
		func(ctx context.Context, o metric.Observer) error {
			o.ObserveFloat64(cpuTempGauge, randomTemp())
			// as there is no async histogram, use this callback to also record the other meters
			hdErrorsMeter.Add(ctx, 1, metric.WithAttributes(hdErrorsAttributeSdb))
			cpuEnergyMeter.Record(ctx, randomEnergy(), metric.WithAttributes(cpuEnergyAttribute1))

			return nil
		},
		cpuTempGauge,
	)
	if err != nil {
		return fmt.Errorf("error registering callback: %w", err)
	}

	requestURLParamsMeter, err = meter.Int64Counter(
		"promhttp.metric.handler.requests.url_params",
		metric.WithDescription("Total number of requests to the /metrics endpoint with URL parameters."),
		metric.WithUnit("{requests}"),
	)
	if err != nil {
		return fmt.Errorf("error creating promhttp.metric.handler.requests.url_params counter: %w", err)
	}

	return nil
}

func initTerminateEndpoint(ctx context.Context) {
	var ok bool
	if terminateEndpoint, ok = os.LookupEnv("TERMINATE_ENDPOINT"); !ok {
		terminateEndpoint = fmt.Sprintf("localhost:%d", serverPort)
	}

	logger.InfoContext(ctx, "Using terminate endpoint: "+terminateEndpoint)
}

// randomTemp generates the temperature ranging from 60 to 90
func randomTemp() float64 {
	return math.Round(rand.Float64()*300)/10 + 60 //nolint:gosec,mnd // G404: Use of weak random number generator
}

// randomEnergy generates the energy ranging from 0 to 100
func randomEnergy() float64 {
	const maxEnergy = 100.0
	return math.Round(rand.Float64() * maxEnergy) //nolint:gosec // G404: Use of weak random number generator
}

// randBool generates a random bool
func randBool() bool {
	return rand.Intn(2) == 1 //nolint:gosec,mnd // G404: Use of weak random number generator
}

// forwardHandler handles the incoming request by forwarding it to the terminate endpoint
func forwardHandler(w http.ResponseWriter, r *http.Request) {
	// Initialize a new span for the current trace
	ctx, span := tracer.Start(r.Context(), "forward")
	defer span.End()

	// Forward the request to the terminate endpoint using the instrumented HTTP client, so that trace context gets propagated
	requestURL := fmt.Sprintf("http://%s/terminate", terminateEndpoint)

	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		handleError(ctx, w, span, err, "error creating request")
		return
	}

	res, err := client.Do(req)
	if err != nil {
		handleError(ctx, w, span, err, "exception in client call")
		return
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		handleError(ctx, w, span, err, "client call returned malformed body")
		return
	}

	if !isSuccess(res.StatusCode) {
		logger.ErrorContext(ctx, "client: received error response code", slog.String("status", res.Status), slog.String("traceId", span.SpanContext().TraceID().String()))
		span.SetStatus(codes.Error, "error")
	} else {
		logger.InfoContext(ctx, "Forwarded successful", slog.String("status", res.Status), slog.String("traceId", span.SpanContext().TraceID().String()))
	}

	w.WriteHeader(res.StatusCode)
	fmt.Fprint(w, string(resBody))
}

func handleError(ctx context.Context, w http.ResponseWriter, span trace.Span, err error, message string) {
	logger.ErrorContext(ctx, "client: "+message, slog.String("error", err.Error()), slog.String("traceId", span.SpanContext().TraceID().String()))
	span.RecordError(err)
	span.SetStatus(codes.Error, message)
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, "Error")
}

func isSuccess(status int) bool {
	return status >= http.StatusOK && status < http.StatusMultipleChoices
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

		const asRandomAsItGets = 5
		hdErrorsMeter.Add(ctx, asRandomAsItGets, metric.WithAttributes(hdErrorsAttributeSda))

		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error")

		logger.ErrorContext(ctx, "Terminated errorful", slog.String("traceId", span.SpanContext().TraceID().String()))

		return
	}

	logger.InfoContext(ctx, "Terminated successful", slog.String("traceId", span.SpanContext().TraceID().String()))

	const notQuiteAsRandomAsItGets = 1
	hdErrorsMeter.Add(ctx, notQuiteAsRandomAsItGets, metric.WithAttributes(hdErrorsAttributeSda))
	fmt.Fprintf(w, "Success")
}

func run() error {
	ctx := context.Background()

	logger.InfoContext(ctx, "Setting up OTel SDK")

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
		if err := tp.Shutdown(ctx); err != nil {
			logger.ErrorContext(ctx, "Error shutting down trace provider", slog.String("error", err.Error()))
		}

		if err := mp.Shutdown(ctx); err != nil {
			logger.ErrorContext(ctx, "Error shutting down meter provider", slog.String("error", err.Error()))
		}
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

	otelSDKLogger, err := newOTelSDKLogger(ctx)
	if err != nil {
		return fmt.Errorf("error creating OTel SDK logger: %w", err)
	}

	otel.SetLogger(*otelSDKLogger)

	// Configure the HTTP server
	http.DefaultClient.Timeout = clientTimeout

	// Initialize the forward handler with the terminate endpoint to use
	initTerminateEndpoint(ctx)

	// Wrap the handler with OpenTelemetry instrumentation
	wrappedForwardHandler := otelhttp.NewHandler(http.HandlerFunc(forwardHandler), "auto-forward")
	wrappedTerminateHandler := otelhttp.NewHandler(http.HandlerFunc(terminateHandler), "auto-terminate")

	// Register the handlers
	http.Handle("/forward", wrappedForwardHandler)
	http.Handle("/terminate", wrappedTerminateHandler)
	http.Handle("/", wrappedForwardHandler)

	// Register metrics endpoint in case a prometheus exporter is used
	http.Handle("/metrics", otelhttp.NewHandler(
		newURLParamCounterMiddleware(
			promhttp.Handler(),
		),
		"metrics"))

	// Start the HTTP server
	logger.InfoContext(ctx, "Starting server on port "+strconv.Itoa(serverPort))

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", serverPort),
		ReadHeaderTimeout: serverReadTimeout,
	}

	err = server.ListenAndServe()
	if err != nil {
		return fmt.Errorf("error starting server: %w", err)
	}

	return nil
}

// newURLParamCounterMiddleware is a middleware that counts the number of URL parameters passed to the /metrics request.
// It is used to test prometheus.io/param_{name}={value} annotation.
func newURLParamCounterMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urlParams := r.URL.Query()
		for name := range urlParams {
			requestURLParamsMeter.Add(r.Context(), 1,
				metric.WithAttributes(
					attribute.String("name", name),
					attribute.String("value", urlParams.Get(name)),
				),
			)
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	if err := run(); err != nil {
		logger.ErrorContext(context.Background(), "Error running server", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
