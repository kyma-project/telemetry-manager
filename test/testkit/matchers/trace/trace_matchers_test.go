package trace

import (
	"testing"

	. "github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var fts = []FlatTrace{
	{
		Name: "ingress",
		ResourceAttributes: map[string]string{
			"service.name":        "backend",
			"k8s.pod.ip":          "10.42.1.76",
			"k8s.deployment.name": "backend",
		},
		SpanAttributes: map[string]string{
			"response_size":           "31",
			"upstream_cluster.name":   "inbound|4317||",
			"istio.canonical_service": "backend",
		},
	},
	{
		Name: "ingress-2",
		ResourceAttributes: map[string]string{
			"service.name":        "metric-producer",
			"k8s.pod.ip":          "10.42.1.73",
			"k8s.deployment.name": "istio",
		},
		SpanAttributes: map[string]string{
			"response_size":           "32",
			"upstream_cluster.name":   "inbound|4318||",
			"istio.canonical_service": "istio",
		},
	},
}

func TestFlatTracesMatchers_VerifyInputs(t *testing.T) {
	RegisterTestingT(t)

	td := ptrace.NewTraces()
	Expect(mustMarshalTraces(td)).Should(HaveFlatTraces(ContainElements()), "Should apply matcher to valid trace data")

	Expect([]byte{}).Should(HaveFlatTraces(BeEmpty()), "Should fail when given empty byte slice")

	nilInput, err := HaveFlatTraces(BeEmpty()).Match(nil)
	Expect(err).Should(HaveOccurred(), "Should return error for nil input")
	Expect(nilInput).Should(BeFalse(), "Success should be false for nil input")

	invalidInput, err := HaveFlatTraces(BeEmpty()).Match(struct{}{})
	Expect(err).Should(HaveOccurred(), "should return error for invalid input type")
	Expect(invalidInput).Should(BeFalse(), "Success should be false for invalid input type")
}

func TestFlatTraces_FlatTraceStruct(t *testing.T) {
	RegisterTestingT(t)

	td := ptrace.NewTraces()
	// set resource attributes
	rt := td.ResourceSpans().AppendEmpty()
	attrs := rt.Resource().Attributes()
	attrs.PutStr("service.name", "backend")
	attrs.PutStr("k8s.pod.ip", "10.42.1.76")
	attrs.PutStr("k8s.deployment.name", "backend")

	scope := rt.ScopeSpans().AppendEmpty()

	// set span name
	s := scope.Spans().AppendEmpty()
	s.SetName("ingress")

	// set span attributes
	s.Attributes().PutStr("response_size", "31")
	s.Attributes().PutStr("upstream_cluster.name", "inbound|4317||")
	s.Attributes().PutStr("istio.canonical_service", "backend")

	Expect(mustMarshalTraces(td)).Should(HaveFlatTraces(ContainElements(fts[0])), "Should return a FlatTrace struct with expected values")
}

func TestFlatTracesMatchers(t *testing.T) {
	RegisterTestingT(t)
	Expect(fts).Should(ContainElement(HaveName(Equal("ingress"))), "should have span with name 'ingress'")
	Expect(fts).Should(ContainElement(HaveResourceAttributes(HaveKey("k8s.deployment.name"))), "should have key in resource attributes")
	Expect(fts).Should(ContainElement(HaveSpanAttributes(HaveKey("response_size"))), "should have key in span attributes")
}

func mustMarshalTraces(td ptrace.Traces) []byte {
	var marshaler ptrace.JSONMarshaler

	bytes, err := marshaler.MarshalTraces(td)
	if err != nil {
		panic(err)
	}

	return bytes
}
