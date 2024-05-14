package assert

import (
	"net/http"

	. "github.com/onsi/gomega"

	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/prometheus"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func ExposesOTelCollectorMetrics(proxyClient *apiserverproxy.Client, metricsURL string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(metricsURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		g.Expect(resp).To(HaveHTTPBody(ContainMetricFamily(WithName(ContainSubstring("otelcol")))))

		err = resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}
