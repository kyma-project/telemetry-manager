package verifiers

import (
	"net/http"

	"github.com/onsi/gomega"

	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/apiserver"
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func LogsShouldBeDelivered(proxyClient *apiserver.ProxyClient, logProducerName string, telemetryExportURL string) {
	gomega.Eventually(func(g gomega.Gomega) {
		resp, err := proxyClient.Get(telemetryExportURL)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(gomega.HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(gomega.HaveHTTPBody(gomega.SatisfyAll(
			matchers.ContainLogs(matchers.WithPod(logProducerName)))))
	}, periodic.DefaultTimeout, periodic.TelemetryPollInterval).Should(gomega.Succeed())
}
