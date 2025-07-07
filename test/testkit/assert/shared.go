package assert

import (
	"net/http"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func BackendReachable(t TestingT, backend *kitbackend.Backend) {
	t.Helper()

	queryURL := suite.ProxyClient.ProxyURLForService(backend.Namespace(), backend.Name(), kitbackend.QueryPath, kitbackend.QueryPort)
	HTTPResponseEventuallyMatches(t, queryURL, Not(BeNil()), "Backend should be reachable at %s", queryURL)
}

func BackendDataEventuallyMatches(t TestingT, backend *kitbackend.Backend, httpBodyMatcher types.GomegaMatcher, optionalDescription ...any) {
	t.Helper()

	queryURL := suite.ProxyClient.ProxyURLForService(backend.Namespace(), backend.Name(), kitbackend.QueryPath, kitbackend.QueryPort)
	HTTPResponseEventuallyMatches(t, queryURL, httpBodyMatcher, optionalDescription...)
}

func BackendDataConsistentlyMatches(t TestingT, backend *kitbackend.Backend, httpBodyMatcher types.GomegaMatcher, optionalDescription ...any) {
	t.Helper()

	queryURL := suite.ProxyClient.ProxyURLForService(backend.Namespace(), backend.Name(), kitbackend.QueryPath, kitbackend.QueryPort)
	HTTPResponseConsistentlyMatches(t, queryURL, httpBodyMatcher, optionalDescription...)
}

//nolint:dupl // This function is similar to BackendDataEventuallyMatches but uses Eventually instead of Consistently.
func HTTPResponseEventuallyMatches(t TestingT, queryURL string, httpBodyMatcher types.GomegaMatcher, optionalDescription ...any) {
	t.Helper()

	Eventually(func(g Gomega) {
		resp, err := suite.ProxyClient.GetWithContext(t.Context(), queryURL)
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(httpBodyMatcher), optionalDescription...)
	}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

//nolint:dupl // This function is similar to HTTPResponseEventuallyMatches but uses Consistently instead of Eventually.
func HTTPResponseConsistentlyMatches(t TestingT, queryURL string, httpBodyMatcher types.GomegaMatcher, optionalDescription ...any) {
	t.Helper()

	Consistently(func(g Gomega) {
		resp, err := suite.ProxyClient.GetWithContext(t.Context(), queryURL)
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(httpBodyMatcher), optionalDescription...)
	}, periodic.ConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
}
