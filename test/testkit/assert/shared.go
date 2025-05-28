package assert

import (
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func BackendDataEventuallyMatches(t *testing.T, backend *kitbackend.Backend, httpBodyMatcher types.GomegaMatcher, optionalDescription ...any) {
	t.Helper()

	queryURL := suite.ProxyClient.ProxyURLForService(backend.Namespace(), backend.Name(), kitbackend.QueryPath, kitbackend.QueryPort)
	HTTPResponseEventuallyMatches(t, queryURL, httpBodyMatcher, optionalDescription...)
}

func BackendDataConsistentlyMatches(t *testing.T, backend *kitbackend.Backend, httpBodyMatcher types.GomegaMatcher, optionalDescription ...any) {
	t.Helper()

	queryURL := suite.ProxyClient.ProxyURLForService(backend.Namespace(), backend.Name(), kitbackend.QueryPath, kitbackend.QueryPort)
	HTTPResponseConsistentlyMatches(t, queryURL, httpBodyMatcher, optionalDescription...)
}

func HTTPResponseEventuallyMatches(t *testing.T, queryURL string, httpBodyMatcher types.GomegaMatcher, optionalDescription ...any) {
	t.Helper()

	Eventually(func(g Gomega) {
		resp, err := suite.ProxyClient.GetWithContext(t.Context(), queryURL)
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(httpBodyMatcher), optionalDescription...)
	}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

func HTTPResponseConsistentlyMatches(t *testing.T, queryURL string, httpBodyMatcher types.GomegaMatcher, optionalDescription ...any) {
	t.Helper()

	Consistently(func(g Gomega) {
		resp, err := suite.ProxyClient.GetWithContext(t.Context(), queryURL)
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(httpBodyMatcher), optionalDescription...)
	}, periodic.ConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
}
