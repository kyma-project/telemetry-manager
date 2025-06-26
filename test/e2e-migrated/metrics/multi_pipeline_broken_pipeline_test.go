package metrics

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMultiPipelineBroken(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetrics)

	const (
		endpointKey     = "metric-endpoint"
		brokenHostname  = "metric-rcv-hostname-broken"
		unreachableHost = "http://unreachable:4317"
	)

	var (
		uniquePrefix        = unique.Prefix()
		healthyPipelineName = uniquePrefix("healthy")
		brokenPipelineName  = uniquePrefix("broken")
		backendNs           = uniquePrefix("backend")
		genNs               = uniquePrefix("gen")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)
	healthyPipeline := testutils.NewMetricPipelineBuilder().
		WithName(healthyPipelineName).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()

	unreachableHostSecret := kitk8s.NewOpaqueSecret(brokenHostname, kitkyma.DefaultNamespaceName,
		kitk8s.WithStringData(endpointKey, unreachableHost))
	brokenPipeline := testutils.NewMetricPipelineBuilder().
		WithName(brokenPipelineName).
		WithOTLPOutput(testutils.OTLPEndpointFromSecret(unreachableHostSecret.Name(), unreachableHostSecret.Namespace(), endpointKey)).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		unreachableHostSecret.K8sObject(),
		&healthyPipeline,
		&brokenPipeline,
		telemetrygen.NewPod(genNs, telemetrygen.SignalTypeMetrics).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

	assert.MetricPipelineHealthy(t.Context(), healthyPipelineName)
	assert.MetricPipelineHealthy(t.Context(), brokenPipelineName)
	assert.DeploymentReady(t.Context(), kitkyma.MetricGatewayName)
	assert.DeploymentReady(t.Context(), backend.NamespacedName())
	assert.MetricsFromNamespaceDeliveredWithT(t, backend, genNs, telemetrygen.MetricNames)
}
