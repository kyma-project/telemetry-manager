package gateway

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestEnrichmentValuesPredefined(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelLogGateway)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)
	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithIncludeNamespaces(genNs).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
		Build()

	// All attributes in the enrichment flow are set to predefined values
	generator := telemetrygen.NewPod(
		genNs,
		telemetrygen.SignalTypeLogs,
		telemetrygen.WithResourceAttribute("cloud.availability_zone", "predefined-availability-zone"),
		telemetrygen.WithResourceAttribute("cloud.provider", "predefined-provider"),
		telemetrygen.WithResourceAttribute("cloud.region", "predefined-region"),
		telemetrygen.WithResourceAttribute("host.arch", "predefined-arch"),
		telemetrygen.WithResourceAttribute("host.type", "predefined-type"),
		telemetrygen.WithResourceAttribute("k8s.cluster.name", "predefined-cluster-name"),
		telemetrygen.WithResourceAttribute("k8s.cluster.uid", "predefined-cluster-uid"),
		telemetrygen.WithResourceAttribute("k8s.cronjob.name", "predefined-cronjob-name"),
		telemetrygen.WithResourceAttribute("k8s.daemonset.name", "predefined-daemonset-name"),
		telemetrygen.WithResourceAttribute("k8s.deployment.name", "predefined-deployment-name"),
		telemetrygen.WithResourceAttribute("k8s.job.name", "predefined-job-name"),
		// telemetrygen.WithResourceAttribute("k8s.namespace.name", "predefined-namespace-name"), // this one can't be set as it affects the test's namespace isolation itself
		telemetrygen.WithResourceAttribute("k8s.node.name", "predefined-node-name"),
		telemetrygen.WithResourceAttribute("k8s.pod.name", "predefined-pod-name"),
		telemetrygen.WithResourceAttribute("k8s.statefulset.name", "predefined-statefulset-name"),
		telemetrygen.WithResourceAttribute("kyma.app_name", "predefined-app-name"),
		telemetrygen.WithResourceAttribute("kyma.input.name", "predefined-input-name"),
		telemetrygen.WithResourceAttribute("kyma.kubernetes_io_app_name", "predefined-kubernetes-io-app-name"),
		telemetrygen.WithResourceAttribute("service.name", "predefined-service-name"),
	)

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(genNs).K8sObject(),
		&pipeline,
		generator.K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DeploymentReady(t, kitkyma.LogGatewayName)
	assert.OTelLogPipelineHealthy(t, pipelineName)
	assert.OTelLogsFromNamespaceDelivered(t, backend, genNs)

	// These attributes should not be enriched by the processors and should thus retain the predefined values
	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveResourceAttributes(HaveKeyWithValue("cloud.availability_zone", Equal("predefined-availability-zone"))),
			HaveResourceAttributes(HaveKeyWithValue("cloud.provider", Equal("predefined-provider"))),
			HaveResourceAttributes(HaveKeyWithValue("cloud.region", Equal("predefined-region"))),
			HaveResourceAttributes(HaveKeyWithValue("host.arch", Equal("predefined-arch"))),
			HaveResourceAttributes(HaveKeyWithValue("host.type", Equal("predefined-type"))),
			HaveResourceAttributes(HaveKeyWithValue("k8s.cluster.name", Equal("predefined-cluster-name"))),
			HaveResourceAttributes(HaveKeyWithValue("k8s.cluster.uid", Equal("predefined-cluster-uid"))),
			HaveResourceAttributes(HaveKeyWithValue("k8s.cronjob.name", Equal("predefined-cronjob-name"))),
			HaveResourceAttributes(HaveKeyWithValue("k8s.daemonset.name", Equal("predefined-daemonset-name"))),
			HaveResourceAttributes(HaveKeyWithValue("k8s.deployment.name", Equal("predefined-deployment-name"))),
			HaveResourceAttributes(HaveKeyWithValue("k8s.job.name", Equal("predefined-job-name"))),
			// HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", Equal("predefined-namespace-name"))),
			HaveResourceAttributes(HaveKeyWithValue("k8s.node.name", Equal("predefined-node-name"))),
			HaveResourceAttributes(HaveKeyWithValue("k8s.pod.name", Equal("predefined-pod-name"))),
			HaveResourceAttributes(HaveKeyWithValue("k8s.statefulset.name", Equal("predefined-statefulset-name"))),
			HaveResourceAttributes(HaveKeyWithValue("service.name", Equal("predefined-service-name"))),
		))),
	)

	// These attributes should be dropped by the processors
	assert.BackendDataConsistentlyMatches(t, backend,
		Not(HaveFlatLogs(ContainElement(SatisfyAny(
			HaveResourceAttributes(HaveKey("kyma.app_name")),
			HaveResourceAttributes(HaveKey("kyma.input.name")),
			HaveResourceAttributes(HaveKey("kyma.kubernetes_io_app_name")),
		)))),
	)
}
