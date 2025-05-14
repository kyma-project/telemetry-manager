package agent

import (
	"context"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
	. "github.com/onsi/gomega"
	"io"
	"net/http"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"testing"
)

func TestObservedTime(t *testing.T) {
	RegisterTestingT(t)
	var (
		uniquePrefix = unique.Prefix("agent")
		pipelineName = uniquePrefix()
		genNs        = uniquePrefix("gen")
		backendNs    = uniquePrefix("backend")
	)
	backend := backend.New(backendNs, backend.SignalTypeLogsOTel)
	backendExportURL := backend.ExportURL(suite.ProxyClient)

	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithApplicationInput(true).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()

	var resources []client.Object
	resources = append(resources,
		kitk8s.NewNamespace(genNs).K8sObject(),
		kitk8s.NewNamespace(backendNs).K8sObject(),
		&pipeline,
		loggen.New(genNs).K8sObject(),
	)
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	assert.DeploymentReady(suite.Ctx, suite.K8sClient, kitkyma.LogGatewayName)
	assert.DaemonSetReady(suite.Ctx, suite.K8sClient, kitkyma.LogAgentName)
	assert.OTelLogPipelineHealthy(suite.Ctx, suite.K8sClient, pipelineName)
	assert.DeploymentReady(suite.Ctx, suite.K8sClient, backend.NamespacedName())

	assert.OTelLogsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, genNs)
	assert.DataConsistentlyMatching(suite.ProxyClient, backendExportURL,
		HaveFlatOTelLogs(ContainElement(SatisfyAll(
			HaveOtelTimestamp(Not(BeEmpty())),
			HaveObservedTimestamp(Not(Equal("1970-01-01 00:00:00 +0000 UTC"))),
		))))
	Consistently(func(g Gomega) {
		resp, err := suite.ProxyClient.Get(backendExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		bodyContent, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(bodyContent).To(HaveFlatOTelLogs(ContainElement(SatisfyAll(
			HaveOtelTimestamp(Not(BeEmpty())),
			HaveObservedTimestamp(Not(Equal("1970-01-01 00:00:00 +0000 UTC"))),
		))))
	}, periodic.ConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())

}
