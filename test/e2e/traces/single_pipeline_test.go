package traces

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
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

func TestSinglePipeline(t *testing.T) {
	tests := []struct {
		name     string
		protocol telemetryv1beta1.OTLPProtocol
		endpoint func(backend *kitbackend.Backend) string
	}{
		{
			name:     "grpc",
			protocol: "grpc",
			endpoint: func(b *kitbackend.Backend) string { return b.EndpointHTTP() },
		},
		{
			name:     "http",
			protocol: "http",
			endpoint: func(b *kitbackend.Backend) string { return b.EndpointOTLPHTTP() },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			suite.SetupTest(t, suite.LabelTraces)

			var (
				uniquePrefix = unique.Prefix(tc.name)
				pipelineName = uniquePrefix()
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeTraces)
			pipeline := testutils.NewTracePipelineBuilder().
				WithName(pipelineName).
				WithOTLPOutput(
					testutils.OTLPEndpoint(tc.endpoint(backend)),
					testutils.OTLPProtocol(tc.protocol),
				).
				Build()

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				&pipeline,
				telemetrygen.NewPod(genNs, telemetrygen.SignalTypeTraces).K8sObject(),
			}
			resources = append(resources, backend.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backend)
			assert.DaemonSetReady(t, kitkyma.OTLPGatewayName)
			assert.TracePipelineHealthy(t, pipelineName)
			assert.TracesFromNamespaceDelivered(t, backend, genNs)

			gatewayMetricsURL := suite.ProxyClient.ProxyURLForService(kitkyma.TelemetryOTLPMetricsService.Namespace, kitkyma.TelemetryOTLPMetricsService.Name, "metrics", ports.Metrics)
			assert.EmitsOTelCollectorMetrics(t, gatewayMetricsURL)
		})
	}
}
