package assert

import (
	"context"
	"net/http"
	"time"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/prometheus"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func EmitsFluentBitMetrics(ctx context.Context, metricsURL string) {
	HTTPResponseEventuallyMatches(ctx, metricsURL, HaveFlatMetricFamilies(
		ContainElement(HaveName(ContainSubstring("fluentbit"))),
	))
}

func EmitsOTelCollectorMetrics(ctx context.Context, metricsURL string) {
	HTTPResponseEventuallyMatches(ctx, metricsURL, HaveFlatMetricFamilies(
		ContainElement(HaveName(ContainSubstring("otelcol"))),
	))
}

func EmitsManagerMetrics(ctx context.Context, matchers ...types.GomegaMatcher) {
	Eventually(func(g Gomega) {
		metricsPath := "metrics"
		telemetryManagerMetricsURL := suite.ProxyClient.ProxyURLForService(
			kitkyma.TelemetryManagerMetricsServiceName.Namespace,
			kitkyma.TelemetryManagerMetricsServiceName.Name,
			metricsPath,
			kitkyma.TelemetryManagerMetricsPort)
		resp, err := suite.ProxyClient.GetWithContext(ctx, telemetryManagerMetricsURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		g.Expect(resp).To(HaveHTTPBody(HaveFlatMetricFamilies(ContainElement(SatisfyAll(matchers...)))))

		err = resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())
	}, periodic.EventuallyTimeout, 5*time.Second).Should(Succeed())
}
