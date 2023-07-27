//go:build e2e

package matchers

import (
	"os"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HaveGauges", Label("metrics"), func() {
	var fileBytes []byte
	var expectedMetrics []pmetric.Metric

	BeforeEach(func() {
		m1 := pmetric.NewMetric()
		m1.SetName("room_temperature")
		ts1 := pcommon.NewTimestampFromTime(time.Unix(0, 1682438376750990000))
		gauge1 := m1.SetEmptyGauge()
		dp11 := gauge1.DataPoints().AppendEmpty()
		dp11.SetTimestamp(ts1)
		dp11.SetStartTimestamp(ts1)
		dp11.SetDoubleValue(0.5)

		m2 := pmetric.NewMetric()
		m2.SetName("room_humidity")
		ts2 := pcommon.NewTimestampFromTime(time.Unix(0, 1682438376750991000))
		gauge2 := m2.SetEmptyGauge()
		dp21 := gauge2.DataPoints().AppendEmpty()
		dp21.SetTimestamp(ts2)
		dp21.SetStartTimestamp(ts2)
		dp21.SetDoubleValue(3.5)

		expectedMetrics = []pmetric.Metric{m1, m2}
	})

	Context("with nil input", func() {
		It("should error", func() {
			success, err := HaveMetrics(expectedMetrics...).Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with input of invalid type", func() {
		It("should error", func() {
			success, err := HaveMetrics(expectedMetrics...).Match(struct{}{})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should fail", func() {
			Expect([]byte{}).ShouldNot(HaveMetrics(expectedMetrics...))
		})
	})

	Context("with no metrics matching the expecting metrics", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/have_metrics/no_match.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail", func() {
			Expect(fileBytes).ShouldNot(HaveMetrics(expectedMetrics...))
		})
	})

	Context("with some metrics matching the expecting metrics", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/have_metrics/partial_match.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail", func() {
			Expect(fileBytes).ShouldNot(HaveMetrics(expectedMetrics...))
		})
	})

	Context("with all metrics matching the expecting metrics", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/have_metrics/full_match.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed", func() {
			Expect(fileBytes).Should(HaveMetrics(expectedMetrics...))
		})
	})

	Context("with invalid input", func() {
		BeforeEach(func() {
			fileBytes = []byte{1, 2, 3}
		})

		It("should error", func() {
			success, err := HaveMetrics(expectedMetrics...).Match(fileBytes)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})
})

var _ = Describe("HaveSumMetrics", Label("metrics"), func() {
	var fileBytes []byte
	var expectedMetrics []pmetric.Metric

	BeforeEach(func() {
		m1 := pmetric.NewMetric()
		m1.SetName("room_temperature")
		ts1 := pcommon.NewTimestampFromTime(time.Unix(0, 1682438376750990000))
		sum1 := m1.SetEmptySum()
		dp11 := sum1.DataPoints().AppendEmpty()
		dp11.SetTimestamp(ts1)
		dp11.SetStartTimestamp(ts1)
		dp11.SetDoubleValue(0.5)
		sum1.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

		m2 := pmetric.NewMetric()
		m2.SetName("room_humidity")
		ts2 := pcommon.NewTimestampFromTime(time.Unix(0, 1682438376750991000))
		sum2 := m2.SetEmptySum()
		dp21 := sum2.DataPoints().AppendEmpty()
		dp21.SetTimestamp(ts2)
		dp21.SetStartTimestamp(ts2)
		dp21.SetDoubleValue(3.5)
		sum2.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

		expectedMetrics = []pmetric.Metric{m1, m2}
	})

	Context("with nil input", func() {
		It("should error", func() {
			success, err := HaveSumMetrics(expectedMetrics...).Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with input of invalid type", func() {
		It("should error", func() {
			success, err := HaveSumMetrics(expectedMetrics...).Match(struct{}{})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should fail", func() {
			Expect([]byte{}).ShouldNot(HaveSumMetrics(expectedMetrics...))
		})
	})

	Context("with no metrics matching the expecting metrics", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/have_metrics_sum/no_match.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail", func() {
			Expect(fileBytes).ShouldNot(HaveSumMetrics(expectedMetrics...))
		})
	})

	Context("with some metrics matching the expecting metrics", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/have_metrics_sum/partial_match.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail", func() {
			Expect(fileBytes).ShouldNot(HaveSumMetrics(expectedMetrics...))
		})
	})

	Context("with all metrics matching the expecting metrics", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/have_metrics_sum/full_match.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed", func() {
			Expect(fileBytes).Should(HaveSumMetrics(expectedMetrics...))
		})
	})

	Context("with invalid input", func() {
		BeforeEach(func() {
			fileBytes = []byte{1, 2, 3}
		})

		It("should error", func() {
			success, err := HaveSumMetrics(expectedMetrics...).Match(fileBytes)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})
})

var _ = Describe("HaveNumberOfMetrics", Label("metrics"), func() {
	Context("with nil input", func() {
		It("should match 0", func() {
			success, err := HaveNumberOfMetrics(0).Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		var emptyMetrics []pmetric.Metric
		It("should match 0", func() {
			success, err := HaveNumberOfMetrics(0).Match(emptyMetrics)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with input of invalid type", func() {
		It("should error", func() {
			success, err := HaveNumberOfMetrics(0).Match(struct{}{})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with having metrics", func() {
		var fileBytes []byte
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/have_metrics/full_match.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed", func() {
			Expect(fileBytes).Should(HaveNumberOfMetrics(2))
		})
	})
})

var _ = Describe("HaveAttributes", Label("metrics"), func() {
	var fileBytes []byte
	var expectedMetricAttributes []string

	BeforeEach(func() {
		expectedMetricAttributes = []string{"k8s.cluster.name", "k8s.container.name", "k8s.daemonset.name", "k8s.deployment.name", "k8s.namespace.name", "k8s.node.name", "k8s.pod.name", "k8s.pod.uid", "kyma.source"}
	})

	Context("with nil input", func() {
		It("should error", func() {
			success, err := HaveAttributes(expectedMetricAttributes...).Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with input of invalid type", func() {
		It("should error", func() {
			success, err := HaveAttributes(expectedMetricAttributes...).Match(struct{}{})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should fail", func() {
			Expect([]byte{}).ShouldNot(HaveAttributes(expectedMetricAttributes...))
		})
	})

	Context("with no attribute matching the expecting attributes", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/have_metrics/no_match.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail", func() {
			Expect(fileBytes).ShouldNot(HaveAttributes(expectedMetricAttributes...))
		})
	})

	Context("with all attributes matching the expecting attributes", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/kubelet_metrics/kubelet_metrics.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed", func() {
			Expect(fileBytes).Should(HaveAttributes(expectedMetricAttributes...))
		})
	})

	Context("with invalid input", func() {
		BeforeEach(func() {
			fileBytes = []byte{1, 2, 3}
		})

		It("should error", func() {
			success, err := HaveAttributes(expectedMetricAttributes...).Match(fileBytes)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})
})

var _ = Describe("HaveMetricNames", Label("metrics"), func() {
	var fileBytes []byte
	var expectedMetricNames []string

	BeforeEach(func() {
		expectedMetricNames = []string{"container.cpu.time", "container.cpu.utilization", "container.filesystem.available", "container.filesystem.capacity", "container.filesystem.usage", "container.memory.available"}
	})

	Context("with nil input", func() {
		It("should error", func() {
			success, err := HaveMetricNames(expectedMetricNames...).Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with input of invalid type", func() {
		It("should error", func() {
			success, err := HaveMetricNames(expectedMetricNames...).Match(struct{}{})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should fail", func() {
			Expect([]byte{}).ShouldNot(HaveMetricNames(expectedMetricNames...))
		})
	})

	Context("with no metric name matching the expecting metric names", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/have_metrics/no_match.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail", func() {
			Expect(fileBytes).ShouldNot(HaveMetricNames(expectedMetricNames...))
		})
	})

	Context("with all metric names matching the expecting metric names", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/kubelet_metrics/kubelet_metrics.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed", func() {
			Expect(fileBytes).Should(HaveMetricNames(expectedMetricNames...))
		})
	})

	Context("with invalid input", func() {
		BeforeEach(func() {
			fileBytes = []byte{1, 2, 3}
		})

		It("should error", func() {
			success, err := HaveMetricNames(expectedMetricNames...).Match(fileBytes)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})
})
