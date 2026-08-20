package traces

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

// TestPassthroughEndpoint verifies that the passthrough:/// gRPC resolver scheme is accepted
// end-to-end. This scheme bypasses DNS resolution and passes the address directly to the OS,
// which avoids the Happy Eyeballs IPv6-first behavior introduced by grpc-go v1.77.0 that
// causes 250ms+ latency penalties or connection failures in IPv4-only environments.
//
// Unlike http://, the passthrough:/// scheme defaults to TLS (insecure: false).
// Use TLS.Insecure: true to connect to a plaintext backend.
func TestPassthroughEndpoint(t *testing.T) {
	suite.SetupTest(t, suite.LabelTraces)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeTraces)
	pipeline := testutils.NewTracePipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(
			testutils.OTLPEndpoint(fmt.Sprintf("passthrough:///%s", backend.EndpointNoScheme())),
			testutils.OTLPProtocol("grpc"),
			testutils.OTLPInsecure(true),
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
}
