package metricpipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func batchQueryPrometheus(ctx context.Context) error {
	queries := []string{
		"rate(otelcol_exporter_send_failed_metric_points[5m]) > 0",
		"rate(otelcol_exporter_enqueue_failed_metric_points[5m]) > 0",
		"(otelcol_exporter_queue_size/otelcol_exporter_queue_capacity)*100 > 90",

		"rate(otelcol_processor_dropped_metric_points[5m]) > 0",
		"rate(otelcol_processor_refused_metric_points[5m]) > 0",

		"rate(otelcol_receiver_refused_metric_points[5m]) > 0",
	}

	start := time.Now()
	for _, query := range queries {
		if err := queryPrometheus(ctx, query); err != nil {
			return fmt.Errorf("failed to perform query: %s", query)
		}
	}
	elapased := time.Since(start)

	logf.FromContext(ctx).Info("Prometheus batch query succeeded!", "elapsed_ms", elapased.Milliseconds())

	return nil
}

func queryPrometheus(ctx context.Context, query string) error {
	apiURL := "http://prometheus-server.default:80"
	client, err := api.NewClient(api.Config{
		Address: apiURL,
	})
	if err != nil {
		return fmt.Errorf("failed to create Prometheus client: %w", err)
	}

	v1api := v1.NewAPI(client)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, _, err = v1api.Query(ctx, query, time.Now(), v1.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("failed to query Prometheus: %w", err)
	}

	return nil
}
