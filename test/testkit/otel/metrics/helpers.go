package metrics

import (
	"context"
	"fmt"

	"github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
)

func MakeAndSendGaugeMetrics(proxyClient *apiserverproxy.Client, otlpPushURL string) []pmetric.Metric {
	builder := NewBuilder()
	var gauges []pmetric.Metric
	for i := 0; i < 50; i++ {
		gauge := NewGauge()
		gauges = append(gauges, gauge)
		builder.WithMetric(gauge)
	}
	gomega.Expect(sendGaugeMetrics(context.Background(), proxyClient, builder.Build(), otlpPushURL)).To(gomega.Succeed())

	return gauges
}

func sendGaugeMetrics(ctx context.Context, proxyClient *apiserverproxy.Client, metrics pmetric.Metrics, otlpPushURL string) error {
	sender, err := NewHTTPExporter(otlpPushURL, proxyClient)
	if err != nil {
		return fmt.Errorf("unable to create an OTLP HTTP Metric Exporter instance: %w", err)
	}
	return sender.ExportMetrics(ctx, metrics)
}
