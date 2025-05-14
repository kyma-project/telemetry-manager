package agent

import (
	"io"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestAttributesParser(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelLogAgent)

	var (
		uniquePrefix = unique.Prefix()
		generatorNs  = uniquePrefix("generator")
		backendNs    = uniquePrefix("backend")
		pipelineName = uniquePrefix()
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)
	backendExportURL := backend.ExportURL(suite.ProxyClient)
	hostSecretRef := backend.HostSecretRefV1Alpha1()

	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithApplicationInput(true).
		WithOTLPOutput(
			testutils.OTLPEndpointFromSecret(
				hostSecretRef.Name,
				hostSecretRef.Namespace,
				hostSecretRef.Key,
			),
		).
		Build()

	logProducer := loggen.New(generatorNs).WithUseJSON().K8sObject()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(generatorNs).K8sObject(),
		logProducer,
		&pipeline,
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(t.Context(), suite.K8sClient, resources...))
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	assert.DaemonSetReady(suite.Ctx, suite.K8sClient, kitkyma.LogAgentName)
	assert.DeploymentReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: kitbackend.DefaultName, Namespace: backendNs})
	assert.OTelLogPipelineHealthy(suite.Ctx, suite.K8sClient, pipelineName)
	assert.OTelLogsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, generatorNs)

	assert.DataEventuallyMatching(suite.ProxyClient, backendExportURL, HaveFlatOTelLogs(ContainElement(SatisfyAll(
		HaveOtelTimestamp(Not(BeEmpty())),
		HaveObservedTimestamp(Not(BeEmpty())),
		HaveTraceId(Not(BeEmpty())),
		HaveSpanId(Not(BeEmpty())),
		HaveTraceId(Equal("255c2212dd02c02ac59a923ff07aec74")),
	))))

	assert.DataEventuallyMatching(suite.ProxyClient, backendExportURL, HaveFlatOTelLogs(ContainElement(SatisfyAll(
		HaveOtelTimestamp(Not(BeEmpty())),
		HaveObservedTimestamp(Not(BeEmpty())),
		HaveSpanId(Not(BeEmpty())),
		HaveTraceId(Equal("80e1afed08e019fc1110464cfa66635c")),
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
			HaveObservedTimestamp(Not(BeEmpty())),
			HaveAttributes(Not(HaveKey("trace_id"))),
			HaveAttributes(Not(HaveKey("span_id"))),
			HaveAttributes(Not(HaveKey("trace_flags"))),
			HaveAttributes(Not(HaveKey("traceparent"))),
		))))
	}, periodic.ConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())

	Consistently(func(g Gomega) {
		resp, err := suite.ProxyClient.Get(backendExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		bodyContent, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(bodyContent).To(HaveFlatOTelLogs(ContainElement(SatisfyAll(
			HaveOtelTimestamp(Not(BeEmpty())),
			HaveObservedTimestamp(Not(BeEmpty())),
			HaveTraceId(BeEmpty()),
			HaveSpanId(BeEmpty()),
			HaveAttributes(HaveKey("span_id")),
		))))
	}, periodic.ConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
}
