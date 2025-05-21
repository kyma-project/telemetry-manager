package assert

import (
	"context"
	"net/http"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func BackendDataEventuallyMatches(ctx context.Context, backend *kitbackend.Backend, httpBodyMatcher types.GomegaMatcher) {
	queryURL := suite.ProxyClient.ProxyURLForService(backend.Namespace(), backend.Name(), kitbackend.QueryPath, kitbackend.QueryPort)
	DataEventuallyMatches(ctx, queryURL, httpBodyMatcher)
}

func BackendDataConsistentlyMatches(ctx context.Context, backend *kitbackend.Backend, httpBodyMatcher types.GomegaMatcher) {
	queryURL := suite.ProxyClient.ProxyURLForService(backend.Namespace(), backend.Name(), kitbackend.QueryPath, kitbackend.QueryPort)
	DataConsistentlyMatches(ctx, queryURL, httpBodyMatcher)
}

func DataEventuallyMatches(ctx context.Context, queryURL string, httpBodyMatcher types.GomegaMatcher) {
	Eventually(func(g Gomega) {
		resp, err := suite.ProxyClient.GetWithContext(ctx, queryURL)
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(httpBodyMatcher))
	}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

func DataConsistentlyMatches(ctx context.Context, queryURL string, httpBodyMatcher types.GomegaMatcher) {
	Consistently(func(g Gomega) {
		resp, err := suite.ProxyClient.GetWithContext(ctx, queryURL)
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(httpBodyMatcher))
	}, periodic.ConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
}
