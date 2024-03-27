package verifiers

import (
	. "github.com/onsi/gomega"
	"net/http"

	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/prometheus"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func SelfMonitorWebhookShouldHaveBeenCalled(proxyClient *apiserverproxy.Client) {
	Eventually(func(g Gomega) {
		telemetryManagerMetricsURL := proxyClient.ProxyURLForService(
			kitkyma.TelemetryManagerMetricsServiceName.Namespace,
			kitkyma.TelemetryManagerMetricsServiceName.Name,
			"metrics",
			kitkyma.TelemetryManagerMetricsPort)
		resp, err := proxyClient.Get(telemetryManagerMetricsURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		g.Expect(resp).To(HaveHTTPBody(ContainMetricFamily(WithName(Equal("controller_runtime_webhook_requests_total")))))

		err = resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}
