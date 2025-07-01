package metrics

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestSinglePipelineV1Beta1(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetrics)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)
	pipeline := telemetryv1beta1.MetricPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: pipelineName,
		},
		Spec: telemetryv1beta1.MetricPipelineSpec{
			Output: telemetryv1beta1.MetricPipelineOutput{
				OTLP: &telemetryv1beta1.OTLPOutput{
					Endpoint: telemetryv1beta1.ValueType{
						Value: backend.Endpoint(),
					},
				},
			},
		},
	}

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&pipeline,
		telemetrygen.NewPod(genNs, telemetrygen.SignalTypeMetrics).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

	assert.DeploymentReady(t.Context(), backend.NamespacedName())
	assert.DeploymentReady(t.Context(), kitkyma.MetricGatewayName)
	assert.MetricPipelineHealthy(t.Context(), pipelineName)
	assert.MetricsFromNamespaceDelivered(t, backend, genNs, telemetrygen.MetricNames)

	gatewayMetricsURL := suite.ProxyClient.ProxyURLForService(kitkyma.MetricGatewayMetricsService.Namespace, kitkyma.MetricGatewayMetricsService.Name, "metrics", ports.Metrics)
	assert.EmitsOTelCollectorMetrics(t, gatewayMetricsURL)
}
