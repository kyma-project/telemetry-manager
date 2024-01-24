package verifiers

import (
	"net/http"

	. "github.com/onsi/gomega"

	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/prometheus"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func ShouldExposeCollectorMetrics(proxyClient *apiserverproxy.Client, metricsURL string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(metricsURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		//Take otelcol_process_uptime metric as an example
		g.Expect(resp).To(HaveHTTPBody(ContainPrometheusMetric("otelcol_process_uptime")))

		err = resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}
