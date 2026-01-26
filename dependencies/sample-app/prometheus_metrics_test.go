package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
)

// setupTestEnvironment sets up the OTel SDK with Prometheus exporter for testing,
// reusing all production setup functions from main.
func setupTestEnvironment(t *testing.T) (context.Context, *metricsdk.MeterProvider) {
	t.Helper()

	ctx := t.Context()

	// Use Prometheus exporter for this test
	t.Setenv("OTEL_METRICS_EXPORTER", "prometheus")

	// Create resources using the same setup as main
	res, err := newOtelResource()
	require.NoError(t, err)

	// Create metric reader using the same setup as main
	reader, err := newMetricReader(ctx)
	require.NoError(t, err)

	// Create meter provider using the same setup as main
	meterProvider := newMeterProvider(reader, res)

	t.Cleanup(func() {
		if err := meterProvider.Shutdown(ctx); err != nil {
			t.Errorf("failed to shutdown meter provider: %v", err)
		}
	})

	// Set global meter provider for the test
	otel.SetMeterProvider(meterProvider)

	// Initialize the metrics using the same setup as main
	if err := initMetrics(); err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	return ctx, meterProvider
}

// TestPrometheusMetricNames validates the Prometheus exporter configuration.
func TestPrometheusMetricNames(t *testing.T) {
	ctx, _ := setupTestEnvironment(t)

	// Record some metric values using the global metrics
	hdErrorsMeter.Add(ctx, 5, metric.WithAttributes(attribute.String("device", "/dev/sda")))
	cpuEnergyMeter.Record(ctx, 45.7, metric.WithAttributes(attribute.String("core", "0")))

	// Create a test HTTP server using the same handler setup as main
	server := httptest.NewServer(newURLParamCounterMiddleware(promhttp.Handler()))
	defer server.Close()

	// Make a request with URL parameters to trigger the promhttp metric
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"?foo=bar", nil)
	if err != nil {
		t.Fatalf("failed to create metrics trigger request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to perform metrics trigger request: %v", err)
	}

	defer resp.Body.Close()

	// Now fetch the metrics
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create metrics fetch request: %v", err)
	}

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to perform metrics fetch request: %v", err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	metricsOutput := string(body)
	contentType := resp.Header.Get("Content-Type")

	t.Logf("Content-Type: %s", contentType)

	// Verify 'escaping=underscores' is present
	// This is THE KEY that tells the OTEL receiver to convert underscores back to dots
	//
	// NOTE: Both NoUTF8EscapingWithSuffixes and UnderscoreEscapingWithSuffixes
	// produce this parameter in v0.61.0. This test cannot distinguish between them.
	// What it DOES catch: If someone removes WithTranslationStrategy() entirely,
	// the behavior might change in future SDK versions.
	if !strings.Contains(contentType, "escaping=underscores") {
		t.Fatalf("FAIL: Content-Type missing 'escaping=underscores': %s\n"+
			"Without this, metrics will stay as underscores in the backend!\n"+
			"This means setup.go is NOT configuring an explicit translation strategy.", contentType)
	}

	// Verify suffixes are added (_total, _core, _celsius)
	// This is what "WithSuffixes" strategies do
	requiredMetrics := map[string]string{
		"hd_errors_total{":                                   "hd.errors (counter)",
		"cpu_energy_watt_core":                               "cpu.energy.watt (histogram with unit)",
		"cpu_temperature_celsius":                            "cpu.temperature (gauge with unit)",
		"promhttp_metric_handler_requests_url_params_total{": "promhttp counter",
	}

	for metricName, description := range requiredMetrics {
		if !strings.Contains(metricsOutput, metricName) {
			t.Errorf("FAIL: Metric '%s' not found (from %s)\n"+
				"  This indicates suffixes are not being added!", metricName, description)
		}
	}

	// Verify metrics WITHOUT suffixes are NOT present
	metricsWithoutSuffixes := []string{
		"hd_errors{",       // Should be hd_errors_total
		"cpu_energy_watt{", // Should have _core suffix
		"promhttp_metric_handler_requests_url_params{", // Should have _total
	}

	for _, metricName := range metricsWithoutSuffixes {
		if strings.Contains(metricsOutput, metricName) {
			t.Errorf("FAIL: Found metric WITHOUT required suffix: %s\n"+
				"  This indicates a WithoutSuffixes strategy is active!", metricName)
		}
	}
}
