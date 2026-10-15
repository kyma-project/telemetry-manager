package istio

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/trace"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/trafficgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

const (
	lowSamplingRate = 10.0
)

// TestIstioTracingSampling_ForcedViaTraceparent verifies that sending traceparent with
// sampling=1 forces Envoy to report spans even when randomSamplingPercentage is low.
//
// According to the Istio documentation for randomSamplingPercentage:
//
//	"If a prior sampling decision has been made, that decision will be respected."
//
// This test encodes that documented contract. It is EXPECTED TO FAIL because Envoy's
// connection manager layer makes the sampling decision without reading the traceparent
// sampled flag — the OTel tracer plugin (which does read traceparent) only runs after
// the decision is already made, so sampling=1 cannot force tracing.
//
// Envoy source: source/extensions/tracers/opentelemetry/span_context_extractor.cc
// Envoy source: source/common/http/conn_manager_utility.cc (mutateTracingRequestHeader)
func TestIstioTracingSampling_ForcedViaTraceparent(t *testing.T) {
	suite.SetupTestWithOptions(t, []string{suite.LabelGardener}, kubeprep.WithIstio())

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix("pipeline")
		backendNs    = uniquePrefix("backend")
		appNs        = uniquePrefix("app")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeTraces)
	pipeline := testutils.NewTracePipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
		Build()

	// traceparent with sampling=1: if Istio honors it, spans must appear despite low rate.
	tg := trafficgen.New(appNs).WithHeader("traceparent", "00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-00f067aa0ba902b7-01")

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(appNs, kitk8sobjects.WithIstioInjection()).K8sObject(),
		kitk8sobjects.NewIstioTelemetry("sampling-override", appNs).WithSamplingPercentage(lowSamplingRate).K8sObject(),
		&pipeline,
	}
	resources = append(resources, backend.K8sObjects()...)
	resources = append(resources, tg.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DaemonSetReady(t, kitkyma.OTLPGatewayName)
	assert.TracePipelineHealthy(t, pipelineName)

	// This assertion DOCUMENTS THE EXPECTED (but unimplemented) BEHAVIOR and will fail.
	// At 10% sampling, random sampling alone produces ~0-1 spans per 10 requests.
	// If traceparent sampling=1 were honored, every request would produce a span.
	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatTraces(ContainElement(SatisfyAll(
			HaveSpanAttributes(HaveKeyWithValue("component", "proxy")),
			HaveSpanAttributes(HaveKeyWithValue("istio.namespace", appNs)),
		))),
	)
}

// TestIstioTracingSampling_SuppressionViaTraceparent verifies that sending traceparent
// with sampling=0 suppresses span reporting even when randomSamplingPercentage is 100%.
//
// This is the working direction of the asymmetry: at 100% rate Envoy's Layer 1 always
// decides to trace, which allows Layer 2 (the OTel tracer) to run and honor sampling=0.
func TestIstioTracingSampling_SuppressionViaTraceparent(t *testing.T) {
	suite.SetupTestWithOptions(t, []string{suite.LabelGardener}, kubeprep.WithIstio())

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix("pipeline")
		backendNs    = uniquePrefix("backend")
		appNs        = uniquePrefix("app")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeTraces)
	pipeline := testutils.NewTracePipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
		Build()

	// traceparent with sampling=0: no spans should appear even at 100% sampling rate.
	tg := trafficgen.New(appNs).WithHeader("traceparent", "00-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-00f067aa0ba902b7-00")

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(appNs, kitk8sobjects.WithIstioInjection()).K8sObject(),
		kitk8sobjects.NewIstioTelemetry("sampling-override", appNs).WithSamplingPercentage(100.0).K8sObject(),
		&pipeline,
	}
	resources = append(resources, backend.K8sObjects()...)
	resources = append(resources, tg.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DaemonSetReady(t, kitkyma.OTLPGatewayName)
	assert.TracePipelineHealthy(t, pipelineName)

	assert.BackendDataConsistentlyMatches(t, backend,
		Not(HaveFlatTraces(ContainElement(SatisfyAll(
			HaveSpanAttributes(HaveKeyWithValue("component", "proxy")),
			HaveSpanAttributes(HaveKeyWithValue("istio.namespace", appNs)),
		)))),
	)
}
