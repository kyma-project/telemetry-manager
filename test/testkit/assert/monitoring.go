package assert

import (
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"

	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/prometheus"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func EmitsFluentBitMetrics(t *testing.T, metricsURL string) {
	t.Helper()

	HTTPResponseEventuallyMatches(t, metricsURL, HaveFlatMetricFamilies(
		ContainElement(HaveName(ContainSubstring("fluentbit"))),
	))
}

func EmitsOTelCollectorMetrics(t *testing.T, metricsURL string) {
	t.Helper()

	HTTPResponseEventuallyMatches(t, metricsURL, HaveFlatMetricFamilies(
		ContainElement(HaveName(ContainSubstring("otelcol"))),
	))
}

func EmitsManagerMetrics(t *testing.T, matchers ...gomegatypes.GomegaMatcher) {
	t.Helper()

	Eventually(func(g Gomega) {
		metricsPath := "metrics"
		telemetryManagerMetricsURL := suite.ProxyClient.ProxyURLForService(
			kitkyma.TelemetryManagerMetricsServiceName.Namespace,
			kitkyma.TelemetryManagerMetricsServiceName.Name,
			metricsPath,
			kitkyma.TelemetryManagerMetricsPort)
		resp, err := suite.ProxyClient.GetWithContext(t.Context(), telemetryManagerMetricsURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		g.Expect(resp).To(HaveHTTPBody(HaveFlatMetricFamilies(ContainElement(SatisfyAll(matchers...)))))

		err = resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())
	}, periodic.EventuallyTimeout, 5*time.Second).Should(Succeed())
}
