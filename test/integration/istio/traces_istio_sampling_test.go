package istio

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/types"
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

// samplingRequests is the number of distinct requests the source sends, each carrying its own
// traceparent with a distinct trace ID. Sending many makes the two outcomes unambiguous: with a
// low random rate, "all forced through" (20) stands clearly apart from "a random handful" (~2).
const samplingRequests = 20

// These tests verify that the W3C traceparent sampled flag overrides Istio's
// randomSamplingPercentage, as the Istio reference documents for randomSamplingPercentage:
//
//	"If a prior sampling decision has been made, that decision will be respected. However, if no
//	 sampling decision has been made [...] the traffic will be selected for telemetry generation
//	 at the percentage specified."
//
// Both tests drive real pod-to-pod traffic through the Istio sidecars (via trafficgen) so the
// kyma-traces OpenTelemetry provider path is exercised exactly as a user's workload hits it, and
// assert on the trace IDs that reach the trace backend.

// TestIstioTracingSampling_ForceSampledViaTraceparent verifies that an inbound traceparent with
// the sampled flag set (flags=01) forces tracing even when randomSamplingPercentage is low: all
// requests are reported regardless of the random rate.
func TestIstioTracingSampling_ForceSampledViaTraceparent(t *testing.T) {
	const samplingRate = 10.0

	uniquePrefix := unique.Prefix()

	backend, traceIDs := setupSamplingTest(t, uniquePrefix, samplingRate, "01")

	// Every forced request must be reported: at a 10% random rate only a random handful would
	// arrive, so observing all 20 proves the sampled=1 flag overrode the rate.
	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatTraces(ContainElements(traceIDMatchers(traceIDs))),
	)
}

// TestIstioTracingSampling_ForceNotSampledViaTraceparent verifies that an inbound traceparent with
// the sampled flag cleared (flags=00) suppresses tracing even when randomSamplingPercentage is
// 100%: none of the requests are reported.
func TestIstioTracingSampling_ForceNotSampledViaTraceparent(t *testing.T) {
	const samplingRate = 100.0

	uniquePrefix := unique.Prefix()

	backend, traceIDs := setupSamplingTest(t, uniquePrefix, samplingRate, "00")

	// None of the suppressed requests may be reported: at a 100% random rate every request would
	// otherwise arrive, so observing none proves the sampled=0 flag overrode the rate.
	assert.BackendDataConsistentlyMatches(t, backend,
		Not(HaveFlatTraces(ContainElement(SatisfyAny(traceIDMatchers(traceIDs)...)))),
	)
}

// setupSamplingTest provisions a namespace-scoped Istio Telemetry CR at the given sampling rate,
// a trace pipeline with a backend, and a source/destination traffic generator that sends one
// request per traceparent (each with a distinct trace ID and the given trace-flags byte: "01" =
// sampled, "00" = not sampled). It returns the backend and the sent trace IDs for assertions.
func setupSamplingTest(t *testing.T, uniquePrefix func(...string) string, samplingRate float64, flagByte string) (*kitbackend.Backend, []string) {
	t.Helper()

	suite.SetupTestWithOptions(t, []string{suite.LabelGardener}, kubeprep.WithIstio())

	pipelineName := uniquePrefix("pipeline")
	backendNs := uniquePrefix("backend")
	appNs := uniquePrefix("app")

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeTraces)
	pipeline := testutils.NewTracePipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
		Build()

	traceparents, traceIDs := makeTraceparents(flagByte)
	tg := trafficgen.New(appNs).WithTraceparentHeaders(traceparents)

	firstWave := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(appNs, kitk8sobjects.WithIstioInjection()).K8sObject(),
		kitk8sobjects.NewIstioTelemetry("sampling-override", appNs).WithSamplingPercentage(samplingRate).K8sObject(),
		&pipeline,
	}
	firstWave = append(firstWave, backend.K8sObjects()...)
	firstWave = append(firstWave, tg.Destination().K8sObjects()...)
	Expect(kitk8s.CreateObjects(t, firstWave...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DaemonSetReady(t, kitkyma.OTLPGatewayName)
	assert.TracePipelineHealthy(t, pipelineName)
	assert.PodReady(t, types.NamespacedName{Name: "destination", Namespace: appNs})

	// The destination is ready, so start the source. It waits for the destination to return HTTP
	// 200 before sending the measured requests (see trafficgen), so none are lost to a race.
	Expect(kitk8s.CreateObjects(t, tg.Source().K8sObjects()...)).To(Succeed())
	assert.PodReady(t, types.NamespacedName{Name: "source", Namespace: appNs})

	return backend, traceIDs
}

// makeTraceparents builds samplingRequests W3C traceparent headers, each with a distinct,
// deterministic trace ID and the given trace-flags byte ("01" = sampled, "00" = not sampled).
// It returns the headers and the matching trace IDs (lowercase hex, as the backend reports them).
func makeTraceparents(flagByte string) ([]string, []string) {
	traceparents := make([]string, samplingRequests)
	traceIDs := make([]string, samplingRequests)

	for i := range samplingRequests {
		traceID := fmt.Sprintf("aaaaaaaaaaaaaaaaaaaaaaaa%08x", i)
		traceparents[i] = fmt.Sprintf("00-%s-00f067aa0ba902b7-%s", traceID, flagByte)
		traceIDs[i] = traceID
	}

	return traceparents, traceIDs
}

func traceIDMatchers(traceIDs []string) []gomegatypes.GomegaMatcher {
	matchers := make([]gomegatypes.GomegaMatcher, len(traceIDs))

	for i, id := range traceIDs {
		matchers[i] = HaveTraceID(id)
	}

	return matchers
}
