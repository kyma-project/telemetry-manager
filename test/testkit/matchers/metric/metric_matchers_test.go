package metric

import (
	"testing"
	"time"

	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

var fms = []FlatMetric{
	{
		Name:               "container.cpu.time",
		Description:        "time of container cpu",
		ScopeName:          "runtime",
		ScopeVersion:       "1.0",
		ScopeAttributes:    map[string]string{"baz": "qux"},
		ResourceAttributes: map[string]string{"k8s.cluster.name": "cluster-01", "k8s.deployment.name": "nginx"},
		MetricAttributes:   map[string]string{"foo": "bar"},
		Type:               "Gauge",
	},
	{
		Name:               "container.cpu.usage",
		Description:        "usage of container cpu",
		ScopeName:          "container",
		ScopeVersion:       "2.0",
		ScopeAttributes:    map[string]string{"bar": "baz"},
		ResourceAttributes: map[string]string{"k8s.cluster.name": "cluster-01", "k8s.deployment.name": "istio"},
		MetricAttributes:   map[string]string{"metricAttr": "valueMetricAttr"},
		Type:               "Gauge",
	},
}

func TestMetric_VerifyMatchers(t *testing.T) {
	suite.RegisterTestCase(t)
	md := pmetric.NewMetrics()
	Expect(mustMarshalMetrics(md)).Should(HaveFlatMetrics(ContainElements()), "Should apply matcher to valid metrics data")

	Expect([]byte{}).Should(HaveFlatMetrics(BeEmpty()), "Should fail when given empty byte slice")

	nilInput, err := HaveFlatMetrics(BeEmpty()).Match(nil)
	Expect(err).Should(HaveOccurred(), "Should return error for nil input")
	Expect(nilInput).Should(BeFalse(), "Success should be false for nil input")

	invalidInput, err := HaveFlatMetrics(BeEmpty()).Match(struct{}{})
	Expect(err).Should(HaveOccurred(), "should return error for invalid input type")
	Expect(invalidInput).Should(BeFalse(), "Success should be false for invalid input type")
}

func TestMetric_FlatMetric(t *testing.T) {
	suite.RegisterTestCase(t)
	md := pmetric.NewMetrics()

	rm := md.ResourceMetrics().AppendEmpty()
	attrs := rm.Resource().Attributes()
	attrs.PutStr("k8s.cluster.name", "cluster-01")
	attrs.PutStr("k8s.deployment.name", "nginx")

	s := rm.ScopeMetrics().AppendEmpty()

	s.Scope().SetName("runtime")
	s.Scope().SetVersion("1.0")
	s.Scope().Attributes().PutStr("baz", "qux")

	m := s.Metrics().AppendEmpty()
	m.SetName("container.cpu.time")
	m.SetDescription("time of container cpu")
	gauge := m.SetEmptyGauge()
	pt := gauge.DataPoints().AppendEmpty()

	pt.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	pt.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	pt.SetDoubleValue(1.5)
	pt.Attributes().PutStr("foo", "bar")

	Expect(mustMarshalMetrics(md)).Should(HaveFlatMetrics(ContainElement(fms[0])))
}

func TestMetric_TestMatchers(t *testing.T) {
	suite.RegisterTestCase(t)
	Expect(fms).Should(HaveUniqueNames(ConsistOf("container.cpu.time", "container.cpu.usage")), "Should return unique names")
	Expect(fms).Should(ContainElement(HaveResourceAttributes(HaveKey("k8s.cluster.name"))), "Should have the specified key in resource attributes")
	Expect(fms).Should(ContainElement(HaveName(ContainSubstring("container"))), "Should return the correct name")
	Expect(fms).Should(ContainElement(HaveType(Equal(pmetric.MetricTypeGauge.String()))), "Should return the correct type")
	Expect(fms).Should(ContainElement(HaveScopeName(ContainSubstring("container"))), "Should contain the specified string in scope name")
	Expect(fms).Should(ContainElement(HaveResourceAttributes(HaveKeys(ContainElements("k8s.cluster.name", "k8s.deployment.name")))), "Should have all the keys within the specified list in resource attributes")
}

func mustMarshalMetrics(md pmetric.Metrics) []byte {
	var marshaler pmetric.JSONMarshaler

	bytes, err := marshaler.MarshalMetrics(md)
	if err != nil {
		panic(err)
	}

	return bytes
}
