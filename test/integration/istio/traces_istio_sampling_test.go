package istio

import (
	"fmt"
	"net/http"
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
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

const (
	// forceSamplingRequests is the number of requests sent with traceparent sampling=1.
	// All of them should appear in the backend if Istio honours the flag — but it doesn't.
	forceSamplingRequests = 20

	// lowSamplingRate is chosen low enough that random sampling alone would not produce
	// forceSamplingRequests spans, so the positive assertion only passes if the
	// traceparent flag is actually honoured.
	lowSamplingRate = 10.0
)

// namespaceScopedTracingCR returns an Istio Telemetry CR scoped to a specific namespace,
// overriding the mesh-wide sampling rate for workloads in that namespace only.
func namespaceScopedTracingCR(namespace string, rate float64) string {
	return fmt.Sprintf(`apiVersion: telemetry.istio.io/v1
kind: Telemetry
metadata:
  name: sampling-override
  namespace: %s
spec:
  tracing:
    - providers:
        - name: kyma-traces
      randomSamplingPercentage: %.2f
`, namespace, rate)
}

// TestIstioTracingSamplingForcedViaTraceparent verifies that sending traceparent with
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
// See: https://sap.stackenterprise.co/questions/96869
// Envoy source: source/extensions/tracers/opentelemetry/span_context_extractor.cc
// Envoy source: source/common/http/conn_manager_utility.cc (mutateTracingRequestHeader)
func TestIstioTracingSamplingForcedViaTraceparent(t *testing.T) {
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

	app := prommetricgen.New(appNs)
	appURL := app.PodURL(suite.ProxyClient)

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(appNs, kitk8sobjects.WithIstioInjection()).K8sObject(),
		&pipeline,
		app.Pod().K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	// Apply a namespace-scoped Telemetry CR with a low sampling rate.
	// This overrides the mesh-wide rate for the app namespace only, without affecting other tests.
	// kitk8s.CreateObjects registered t.Cleanup to delete the namespace, which also removes this CR.
	ctx := t.Context()
	Expect(kubeprep.ApplyYAML(ctx, suite.K8sClient, namespaceScopedTracingCR(appNs, lowSamplingRate))).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DaemonSetReady(t, kitkyma.OTLPGatewayName)
	assert.TracePipelineHealthy(t, pipelineName)

	// Send requests with traceparent sampling=1 (sampled flag set).
	// If Istio honours the flag, all forceSamplingRequests spans must appear in the backend.
	for i := range forceSamplingRequests {
		traceID := fmt.Sprintf("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa%08x", i)
		traceparent := fmt.Sprintf("00-%s-00f067aa0ba902b7-01", traceID)

		Eventually(func(g Gomega) {
			resp, err := suite.ProxyClient.GetWithHeaders(ctx, appURL, map[string]string{
				"traceparent": traceparent,
			})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
			defer resp.Body.Close()
		}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
	}

	// The critical assertion: all forceSamplingRequests proxy spans must be present.
	// With a 10% sampling rate and 20 requests, random sampling produces ~2 spans.
	// If traceparent is honoured, we get 20.
	// This assertion DOCUMENTS THE EXPECTED (but unimplemented) BEHAVIOUR and will fail.
	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatTraces(
			WithTransform(func(fts []FlatTrace) []FlatTrace {
				var matching []FlatTrace
				for _, ft := range fts {
					if ft.SpanAttributes["component"] == "proxy" &&
						ft.SpanAttributes["istio.namespace"] == appNs {
						matching = append(matching, ft)
					}
				}
				return matching
			}, HaveLen(forceSamplingRequests)),
		),
	)
}

// TestIstioTracingSamplingSuppressionViaTraceparent verifies that sending traceparent
// with sampling=0 suppresses span reporting even when randomSamplingPercentage is 100%.
//
// This is the working direction of the asymmetry: at 100% rate Envoy's Layer 1 always
// decides to trace, which allows Layer 2 (the OTel tracer) to run and honour sampling=0.
func TestIstioTracingSamplingSuppressionViaTraceparent(t *testing.T) {
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

	app := prommetricgen.New(appNs)
	appURL := app.PodURL(suite.ProxyClient)

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(appNs, kitk8sobjects.WithIstioInjection()).K8sObject(),
		&pipeline,
		app.Pod().K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	// Apply a namespace-scoped Telemetry CR at 100% so Envoy's Layer 1 always decides to
	// trace, giving Layer 2 (the OTel tracer) a chance to honour the sampling=0 flag.
	// The namespace is deleted in t.Cleanup registered by kitk8s.CreateObjects.
	ctx := t.Context()
	Expect(kubeprep.ApplyYAML(ctx, suite.K8sClient, namespaceScopedTracingCR(appNs, 100.0))).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DaemonSetReady(t, kitkyma.OTLPGatewayName)
	assert.TracePipelineHealthy(t, pipelineName)

	// Send requests with traceparent sampling=0 (sampled flag unset).
	for i := range forceSamplingRequests {
		traceID := fmt.Sprintf("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb%08x", i)
		traceparent := fmt.Sprintf("00-%s-00f067aa0ba902b7-00", traceID)

		Eventually(func(g Gomega) {
			resp, err := suite.ProxyClient.GetWithHeaders(ctx, appURL, map[string]string{
				"traceparent": traceparent,
			})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
			defer resp.Body.Close()
		}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
	}

	// Assert that no Istio proxy spans from the app namespace appear in the backend.
	assert.BackendDataConsistentlyMatches(t, backend,
		Not(HaveFlatTraces(ContainElement(SatisfyAll(
			HaveSpanAttributes(HaveKeyWithValue("component", "proxy")),
			HaveSpanAttributes(HaveKeyWithValue("istio.namespace", appNs)),
		)))),
	)
}
