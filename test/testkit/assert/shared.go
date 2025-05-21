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

func BackendDataEventuallyMatching(ctx context.Context, backend *kitbackend.Backend, httpBodyMatcher types.GomegaMatcher) {
	queryURL := suite.ProxyClient.ProxyURLForService(backend.Namespace(), backend.Name(), kitbackend.QueryPath, kitbackend.QueryPort)
	DataEventuallyMatching(ctx, queryURL, httpBodyMatcher)
}

func BackendDataConsistentlyMatching(ctx context.Context, backend *kitbackend.Backend, httpBodyMatcher types.GomegaMatcher) {
	queryURL := suite.ProxyClient.ProxyURLForService(backend.Namespace(), backend.Name(), kitbackend.QueryPath, kitbackend.QueryPort)
	DataConsistentlyMatching(ctx, queryURL, httpBodyMatcher)
}

func DataEventuallyMatching(ctx context.Context, queryURL string, httpBodyMatcher types.GomegaMatcher) {
	Eventually(func(g Gomega) {
		resp, err := suite.ProxyClient.Get(queryURL)
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(httpBodyMatcher))
	}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

func DataConsistentlyMatching(ctx context.Context, queryURL string, httpBodyMatcher types.GomegaMatcher) {
	Consistently(func(g Gomega) {
		resp, err := suite.ProxyClient.Get(queryURL)
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(httpBodyMatcher))
	}, periodic.ConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
}
