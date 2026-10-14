package istio

import (
	"context"
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

	// samplingRate is chosen low enough that random sampling alone would not produce
	// forceSamplingRequests spans, so a positive assertion would only pass if the
	// traceparent flag is actually honoured.
	samplingRate = 10.0
)

// istioCR returns a mesh-wide Istio Telemetry CR YAML with the given randomSamplingPercentage.
func istioCR(rate float64) string {
	return fmt.Sprintf(`apiVersion: telemetry.istio.io/v1
kind: Telemetry
metadata:
  name: access-config
  namespace: istio-system
spec:
  accessLogging:
    - providers:
        - name: stdout-json
        - name: kyma-logs
  tracing:
    - providers:
        - name: kyma-traces
      randomSamplingPercentage: %.2f
`, rate)
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

	// Set a low global sampling rate so random sampling alone cannot produce all spans.
	ctx := t.Context()
	Expect(kubeprep.ApplyYAML(ctx, suite.K8sClient, istioCR(samplingRate))).To(Succeed())
	t.Cleanup(func() {
		// Restore the default 100% rate used by other integration tests.
		_ = kubeprep.ApplyYAML(context.Background(), suite.K8sClient, istioCR(100.0))
	})

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

	// Assert that all sent traces appear in the backend.
	// This assertion DOCUMENTS THE EXPECTED (but unimplemented) BEHAVIOUR.
	// It will fail because Envoy ignores traceparent sampling=1 at Layer 1.
	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatTraces(
			ContainElement(
				HaveSpanAttributes(HaveKeyWithValue("component", "proxy")),
			),
		),
	)

	// The critical assertion: all forceSamplingRequests spans must be present.
	// With a 10% sampling rate and 20 requests, random sampling produces ~2 spans.
	// If traceparent is honoured, we get 20. This assertion will fail.
	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatTraces(
			WithTransform(func(fts []FlatTrace) []FlatTrace {
				var matching []FlatTrace
				for _, ft := range fts {
					if ft.SpanAttributes["component"] == "proxy" {
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

	// Use 100% sampling rate so Envoy's Layer 1 always decides to trace,
	// giving Layer 2 a chance to honour the sampling=0 flag.
	ctx := t.Context()
	Expect(kubeprep.ApplyYAML(ctx, suite.K8sClient, istioCR(100.0))).To(Succeed())

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
