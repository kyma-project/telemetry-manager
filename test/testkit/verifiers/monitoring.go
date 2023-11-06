package verifiers

import (
	"net/http"

	"github.com/onsi/gomega"

	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/apiserver"
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers/prometheus"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func ShouldExposeCollectorMetrics(proxyClient *apiserver.ProxyClient, metricsURL string) {
	gomega.Eventually(func(g gomega.Gomega) {
		resp, err := proxyClient.Get(metricsURL)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(resp).To(gomega.HaveHTTPStatus(http.StatusOK))

		//Take otelcol_process_uptime metric as an example
		g.Expect(resp).To(gomega.HaveHTTPBody(prometheus.ContainPrometheusMetric("otelcol_process_uptime")))

		err = resp.Body.Close()
		g.Expect(err).NotTo(gomega.HaveOccurred())
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(gomega.Succeed())
}
