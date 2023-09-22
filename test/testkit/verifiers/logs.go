package verifiers

import (
	"net/http"

	. "github.com/onsi/gomega"

	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/apiserver"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers"
)

func LogsShouldBeDelivered(proxyClient *apiserver.ProxyClient, logProducerName string, proxyURL string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(proxyURL)
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
			ContainLogs(WithPod(logProducerName)))))
	}, timeout, interval).Should(Succeed())
}
