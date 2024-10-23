package assert

import (
	"net/http"
	"time"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/prometheus"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func EmitsOTelCollectorMetrics(proxyClient *apiserverproxy.Client, metricsURL string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(metricsURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(HaveFlatMetricFamilies(ContainElement(HaveName(ContainSubstring("otelcol"))))))

		err = resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func ManagerEmitsMetric(
	proxyClient *apiserverproxy.Client,
	matchers ...types.GomegaMatcher) {
	Eventually(func(g Gomega) {
		telemetryManagerMetricsURL := proxyClient.ProxyURLForService(
			kitkyma.TelemetryManagerMetricsServiceName.Namespace,
			kitkyma.TelemetryManagerMetricsServiceName.Name,
			"metrics",
			kitkyma.TelemetryManagerMetricsPort)
		resp, err := proxyClient.Get(telemetryManagerMetricsURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		g.Expect(resp).To(HaveHTTPBody(HaveFlatMetricFamilies(ContainElement(SatisfyAll(matchers...)))))

		err = resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())
	}, periodic.EventuallyTimeout, 5*time.Second).Should(Succeed())
}
