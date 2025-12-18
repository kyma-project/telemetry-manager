package traces

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMultiPipelineBroken(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelTraces)

	var (
		uniquePrefix        = unique.Prefix()
		healthyPipelineName = uniquePrefix("healthy")
		brokenPipelineName  = uniquePrefix("broken")
		backendNs           = uniquePrefix("backend")
		genNs               = uniquePrefix("gen")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeTraces)

	healthyPipeline := testutils.NewTracePipelineBuilder().
		WithName(healthyPipelineName).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
		Build()

	brokenPipeline := testutils.NewTracePipelineBuilder().
		WithName(brokenPipelineName).
		WithOTLPOutput(testutils.OTLPEndpointFromSecret("dummy", "dummy", "dummy")). // broken pipeline ref
		Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(genNs).K8sObject(),
		&healthyPipeline,
		&brokenPipeline,
		telemetrygen.NewPod(genNs, telemetrygen.SignalTypeTraces).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DeploymentReady(t, kitkyma.TraceGatewayName)
	assert.TracePipelineHealthy(t, healthyPipelineName)

	assert.TracePipelineHasCondition(t, brokenPipeline.Name, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonReferencedSecretMissing,
	})
	assert.TracesFromNamespaceDelivered(t, backend, genNs)
}
