package misc

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

// TestPassthroughResolverSetting verifies that setting spec.grpc.passthroughResolver: true in the
// Telemetry CR causes the OTel Collector env vars to use the passthrough:/// resolver scheme for
// all gRPC OTLP exporter endpoints, and that traces are still delivered end-to-end.
//
// This setting is intended for IPv4-only clusters where grpc-go v1.77.0's Happy Eyeballs
// IPv6-first DNS resolver causes connection failures.
func TestPassthroughResolverSetting(t *testing.T) {
	suite.SetupTest(t, suite.LabelTelemetry)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
		telemetry    operatorv1beta1.Telemetry
	)

	// Patch the Telemetry CR to enable passthrough resolver, and restore it after the test.
	kitk8s.PreserveAndScheduleRestoreOfTelemetryResource(t, kitkyma.TelemetryName)

	Eventually(func(g Gomega) {
		g.Expect(suite.K8sClient.Get(t.Context(), kitkyma.TelemetryName, &telemetry)).NotTo(HaveOccurred())
		telemetry.Spec.GRPC = &operatorv1beta1.GRPCSpec{PassthroughResolver: true}
		g.Expect(suite.K8sClient.Update(t.Context(), &telemetry)).NotTo(HaveOccurred())
	}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeTraces)

	// Use a plain host:port endpoint — the passthrough resolver setting should rewrite it.
	pipeline := testutils.NewTracePipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(
			testutils.OTLPEndpoint(backend.EndpointNoScheme()),
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

	// Assert that the generated Secret contains a passthrough:/// endpoint, confirming
	// the automatic rewriting took effect.
	Eventually(func(g Gomega) {
		var secret corev1.Secret
		g.Expect(suite.K8sClient.Get(t.Context(), kitkyma.TelemetryOTLPSecretName, &secret)).NotTo(HaveOccurred())

		// The env var key includes the pipeline name in uppercase, e.g. OTLP_ENDPOINT_TRACEPIPELINE_<NAME>
		upperName := strings.ToUpper(strings.ReplaceAll(pipelineName, "-", "_"))
		envVarKey := fmt.Sprintf("OTLP_ENDPOINT_TRACEPIPELINE_%s", upperName)

		g.Expect(secret.Data).To(HaveKey(envVarKey))
		g.Expect(string(secret.Data[envVarKey])).To(HavePrefix("passthrough:///"),
			"expected endpoint to be rewritten to passthrough:/// form")
	}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())

	// Confirm end-to-end delivery still works with the rewritten endpoint.
	assert.TracesFromNamespaceDelivered(t, backend, genNs)

	gatewayMetricsURL := suite.ProxyClient.ProxyURLForService(kitkyma.TelemetryOTLPMetricsService.Namespace, kitkyma.TelemetryOTLPMetricsService.Name, "metrics", ports.Metrics)
	assert.EmitsOTelCollectorMetrics(t, gatewayMetricsURL)
}
