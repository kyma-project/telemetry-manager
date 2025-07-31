package assert

import (
	"net/http"
	"time"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	"github.com/kyma-project/telemetry-manager/test/testkit"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

type BackendAssertion struct {
	optionalDescription []any
	timeout             time.Duration
	queryInterval       time.Duration
}

type BackendAssertionOption func(*BackendAssertion)

func newBackendAssertion(isConsistently bool, options ...BackendAssertionOption) *BackendAssertion {
	ca := &BackendAssertion{}
	for _, option := range options {
		option(ca)
	}

	if ca.timeout == 0 {
		if isConsistently {
			ca.timeout = periodic.ConsistentlyTimeout
		} else {
			ca.timeout = periodic.EventuallyTimeout
		}
	}

	if ca.queryInterval == 0 {
		ca.queryInterval = periodic.TelemetryInterval
	}

	return ca
}

func WithOptionalDescription(description ...any) BackendAssertionOption {
	return func(ca *BackendAssertion) {
		ca.optionalDescription = description
	}
}

func WithCustomTimeout(timeout time.Duration) BackendAssertionOption {
	return func(ca *BackendAssertion) {
		ca.timeout = timeout
	}
}

func WithCustomQueryInterval(interval time.Duration) BackendAssertionOption {
	return func(ca *BackendAssertion) {
		ca.queryInterval = interval
	}
}

// TODO(TeodorSAP): Refactor this function to directly call BackendDataEventuallyMatches with custom query interval.
func BackendReachable(t testkit.T, backend *kitbackend.Backend) {
	t.Helper()

	const queryInterval = time.Second * 5

	queryURL := suite.ProxyClient.ProxyURLForService(backend.Namespace(), backend.Name(), kitbackend.QueryPath, kitbackend.QueryPort)
	Eventually(func(g Gomega) {
		resp, err := suite.ProxyClient.GetWithContext(t.Context(), queryURL)
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
	}, periodic.EventuallyTimeout, queryInterval).Should(Succeed(), "Backend should be reachable at %s", queryURL)
}

func BackendDataEventuallyMatches(t testkit.T, backend *kitbackend.Backend, httpBodyMatcher types.GomegaMatcher, assertionOptions ...BackendAssertionOption) {
	t.Helper()

	queryURL := suite.ProxyClient.ProxyURLForService(backend.Namespace(), backend.Name(), kitbackend.QueryPath, kitbackend.QueryPort)
	HTTPResponseEventuallyMatches(t, queryURL, httpBodyMatcher, assertionOptions...)
}

func BackendDataConsistentlyMatches(t testkit.T, backend *kitbackend.Backend, httpBodyMatcher types.GomegaMatcher, assertionOptions ...BackendAssertionOption) {
	t.Helper()

	queryURL := suite.ProxyClient.ProxyURLForService(backend.Namespace(), backend.Name(), kitbackend.QueryPath, kitbackend.QueryPort)
	HTTPResponseConsistentlyMatches(t, queryURL, httpBodyMatcher, assertionOptions...)
}

//nolint:dupl // This function is similar to BackendDataEventuallyMatches but uses Eventually instead of Consistently.
func HTTPResponseEventuallyMatches(t testkit.T, queryURL string, httpBodyMatcher types.GomegaMatcher, assertionOptions ...BackendAssertionOption) {
	t.Helper()

	backendAssertion := newBackendAssertion(false, assertionOptions...)

	Eventually(func(g Gomega) {
		resp, err := suite.ProxyClient.GetWithContext(t.Context(), queryURL)
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(httpBodyMatcher), backendAssertion.optionalDescription...)
	}, backendAssertion.timeout, backendAssertion.queryInterval).Should(Succeed())
}

//nolint:dupl // This function is similar to HTTPResponseEventuallyMatches but uses Consistently instead of Eventually.
func HTTPResponseConsistentlyMatches(t testkit.T, queryURL string, httpBodyMatcher types.GomegaMatcher, assertionOptions ...BackendAssertionOption) {
	t.Helper()

	backendAssertion := newBackendAssertion(true, assertionOptions...)

	Consistently(func(g Gomega) {
		resp, err := suite.ProxyClient.GetWithContext(t.Context(), queryURL)
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(httpBodyMatcher), backendAssertion.optionalDescription...)
	}, backendAssertion.timeout, backendAssertion.queryInterval).Should(Succeed())
}
