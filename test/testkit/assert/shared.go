package assert

import (
	"net/http"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func TelemetryDataDelivered(
	proxyClient *apiserverproxy.Client,
	backendExportURL string,
	httpBodyMatcher types.GomegaMatcher,
) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(backendExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(httpBodyMatcher))
	}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

func TelemetryDataNotDelivered(
	proxyClient *apiserverproxy.Client,
	backendExportURL string,
	httpBodyMatcher types.GomegaMatcher,
) {
	Consistently(func(g Gomega) {
		resp, err := proxyClient.Get(backendExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(httpBodyMatcher))
	}, periodic.ConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
}
