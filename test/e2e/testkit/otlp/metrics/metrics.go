//go:build e2e

package metrics

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func NewGauge() pmetric.Metrics {
	totalResourceMetrics := 1
	totalAttributes := 7
	totalPts := 2
	startTime := time.Now()

	md := pmetric.NewMetrics()
	rms := md.ResourceMetrics()
	rms.EnsureCapacity(totalResourceMetrics)
	for i := 0; i < totalResourceMetrics; i++ {
		metric := rms.AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()

		metric.SetName("dummy_gauge")
		metric.SetDescription("dummy-description")
		metric.SetUnit("dummy-units")

		gauge := metric.SetEmptyGauge()
		pts := gauge.DataPoints()
		for i := 0; i < totalPts; i++ {
			pt := pts.AppendEmpty()
			pt.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
			pt.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			pt.SetDoubleValue(1.0)

			for i := 0; i < totalAttributes; i++ {
				k := fmt.Sprintf("pt-label-key-%d", i)
				v := fmt.Sprintf("pt-label-val-%d", i)
				pt.Attributes().PutStr(k, v)
			}
		}
	}
	return md
}

func NewDataSender(otlpPushURL string) (testbed.MetricDataSender, error) {
	typedURL, err := url.Parse(otlpPushURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %v", err)
	}

	host := typedURL.Hostname()
	port, err := strconv.Atoi(typedURL.Port())
	if err != nil {
		return nil, fmt.Errorf("failed to parse port: %v", err)
	}

	if typedURL.Scheme == "grpc" {
		return testbed.NewOTLPMetricDataSender(host, port), nil
	}

	if typedURL.Scheme == "https" {
		return testbed.NewOTLPHTTPMetricDataSender(host, port), nil
	}

	return nil, fmt.Errorf("unsupported url scheme: %s", typedURL.Scheme)
}
