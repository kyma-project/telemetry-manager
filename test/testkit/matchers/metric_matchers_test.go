package matchers

import (
	"go.opentelemetry.io/collector/pdata/pmetric"

	kitmetrics "github.com/kyma-project/telemetry-manager/test/testkit/otlp/metrics"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainMetrics", Label("metrics"), func() {
	Context("with nil input", func() {
		It("should error", func() {
			success, err := ContainMetrics(kitmetrics.NewGauge()).Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with input of invalid type", func() {
		It("should error", func() {
			success, err := ContainMetrics(kitmetrics.NewGauge()).Match(struct{}{})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should fail", func() {
			Expect([]byte{}).ShouldNot(ContainMetrics(kitmetrics.NewGauge()))
		})
	})

	Context("with no metrics matching the expected metrics", func() {
		It("should fail", func() {
			md := pmetric.NewMetrics()
			metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
			metrics.AppendEmpty().SetEmptyGauge()

			Expect(mustMarshalMetrics(md)).ShouldNot(ContainMetrics(kitmetrics.NewGauge()))
		})
	})

	Context("with some metrics matching the expecting metrics", func() {
		It("should succeed", func() {
			md := pmetric.NewMetrics()
			metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
			gauge1 := kitmetrics.NewGauge()
			gauge1.CopyTo(metrics.AppendEmpty())
			gauge2 := kitmetrics.NewGauge()
			gauge2.CopyTo(metrics.AppendEmpty())

			Expect(mustMarshalMetrics(md)).Should(ContainMetrics(gauge1))
		})
	})

	Context("with all metrics matching the expected metrics", func() {
		It("should succeed", func() {
			md := pmetric.NewMetrics()
			metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
			gauge1 := kitmetrics.NewGauge()
			gauge1.CopyTo(metrics.AppendEmpty())
			gauge2 := kitmetrics.NewGauge()
			gauge2.CopyTo(metrics.AppendEmpty())

			Expect(mustMarshalMetrics(md)).Should(ContainMetrics(gauge1, gauge2))
		})
	})

	Context("with cumulative sum metrics matching the expected metrics with flipped temporality", func() {
		It("should succeed", func() {
			md := pmetric.NewMetrics()
			metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
			sum := kitmetrics.NewCumulativeSum()
			sum.CopyTo(metrics.AppendEmpty())

			sum.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

			Expect(mustMarshalMetrics(md)).Should(ContainMetrics(sum))
		})
	})

	Context("with invalid input", func() {
		It("should error", func() {
			success, err := ContainMetrics(kitmetrics.NewGauge()).Match([]byte{1, 2, 3})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})
})

var _ = Describe("ContainMetricsThatSatisfy", Label("metrics"), func() {
	var alwaysTrue = func(metric pmetric.Metric) bool {
		return true
	}

	Context("with nil input", func() {
		It("should error", func() {
			success, err := ContainMetricsThatSatisfy(alwaysTrue).Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with input of invalid type", func() {
		It("should error", func() {
			success, err := ContainMetricsThatSatisfy(alwaysTrue).Match(struct{}{})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should fail", func() {
			Expect([]byte{}).ShouldNot(ContainMetricsThatSatisfy(alwaysTrue))
		})
	})

	Context("with no metrics matching the predicate", func() {
		It("should fail", func() {
			md := pmetric.NewMetrics()
			metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
			gauge := kitmetrics.NewGauge()
			gauge.CopyTo(metrics.AppendEmpty())

			Expect(mustMarshalMetrics(md)).ShouldNot(ContainMetricsThatSatisfy(func(metric pmetric.Metric) bool {
				return metric.Type() == pmetric.MetricTypeHistogram || metric.Type() == pmetric.MetricTypeSum
			}))
		})
	})

	Context("with some metrics matching the predicate", func() {
		It("should succeed", func() {
			md := pmetric.NewMetrics()
			metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
			gauge := kitmetrics.NewGauge()
			gauge.CopyTo(metrics.AppendEmpty())
			sum := kitmetrics.NewCumulativeSum()
			sum.CopyTo(metrics.AppendEmpty())

			Expect(mustMarshalMetrics(md)).Should(ContainMetricsThatSatisfy(func(metric pmetric.Metric) bool {
				return metric.Type() == pmetric.MetricTypeGauge
			}))
			Expect(mustMarshalMetrics(md)).Should(ContainMetricsThatSatisfy(func(metric pmetric.Metric) bool {
				return metric.Name() == gauge.Name()
			}))
		})
	})

	Context("with invalid input", func() {
		It("should error", func() {
			success, err := ContainMetricsThatSatisfy(func(metric pmetric.Metric) bool {
				return true
			}).Match([]byte{1, 2, 3})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})
})

var _ = Describe("ConsistOfNumberOfMetrics", Label("metrics"), func() {
	Context("with nil input", func() {
		It("should match 0", func() {
			success, err := ConsistOfNumberOfMetrics(0).Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		var emptyMetrics []pmetric.Metric
		It("should match 0", func() {
			success, err := ConsistOfNumberOfMetrics(0).Match(emptyMetrics)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with input of invalid type", func() {
		It("should error", func() {
			success, err := ConsistOfNumberOfMetrics(0).Match(struct{}{})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with having metrics", func() {
		It("should succeed", func() {
			md := pmetric.NewMetrics()
			metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
			gauge := kitmetrics.NewGauge()
			gauge.CopyTo(metrics.AppendEmpty())
			sum := kitmetrics.NewCumulativeSum()
			sum.CopyTo(metrics.AppendEmpty())

			Expect(mustMarshalMetrics(md)).Should(ConsistOfNumberOfMetrics(2))
		})
	})
})

var _ = Describe("ConsistOfMetricsWithResourceAttributes", Label("metrics"), func() {
	Context("with nil input", func() {
		It("should error", func() {
			success, err := ConsistOfMetricsWithResourceAttributes("k8s.cluster.name").Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with input of invalid type", func() {
		It("should error", func() {
			success, err := ConsistOfMetricsWithResourceAttributes("k8s.cluster.name").Match(struct{}{})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should error", func() {
			success, err := ConsistOfMetricsWithResourceAttributes("k8s.cluster.name").Match([]byte{})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with no attribute matching the expecting attributes", func() {
		It("should fail", func() {
			md := pmetric.NewMetrics()
			rm := md.ResourceMetrics().AppendEmpty()
			attrs := rm.Resource().Attributes()
			attrs.PutStr("k8s.cluster.name", "cluster-01")
			attrs.PutStr("k8s.deployment.name", "nginx")
			metrics := rm.ScopeMetrics().AppendEmpty().Metrics()
			kitmetrics.NewGauge().CopyTo(metrics.AppendEmpty())
			kitmetrics.NewCumulativeSum().CopyTo(metrics.AppendEmpty())

			Expect(mustMarshalMetrics(md)).ShouldNot(ConsistOfMetricsWithResourceAttributes("k8s.container.name"))
		})
	})

	Context("with some attribute matching the expecting attributes", func() {
		It("should fail", func() {
			md := pmetric.NewMetrics()
			rm := md.ResourceMetrics().AppendEmpty()
			attrs := rm.Resource().Attributes()
			attrs.PutStr("k8s.cluster.name", "cluster-01")
			attrs.PutStr("k8s.deployment.name", "nginx")
			metrics := rm.ScopeMetrics().AppendEmpty().Metrics()
			kitmetrics.NewGauge().CopyTo(metrics.AppendEmpty())
			kitmetrics.NewCumulativeSum().CopyTo(metrics.AppendEmpty())

			Expect(mustMarshalMetrics(md)).ShouldNot(ConsistOfMetricsWithResourceAttributes("k8s.deployment.name"))
		})
	})

	Context("with no attribute matching the expecting attributes", func() {
		It("should succeed", func() {
			md := pmetric.NewMetrics()
			rm := md.ResourceMetrics().AppendEmpty()
			attrs := rm.Resource().Attributes()
			attrs.PutStr("k8s.cluster.name", "cluster-01")
			attrs.PutStr("k8s.deployment.name", "nginx")
			metrics := rm.ScopeMetrics().AppendEmpty().Metrics()
			kitmetrics.NewGauge().CopyTo(metrics.AppendEmpty())
			kitmetrics.NewCumulativeSum().CopyTo(metrics.AppendEmpty())

			Expect(mustMarshalMetrics(md)).Should(ConsistOfMetricsWithResourceAttributes("k8s.cluster.name", "k8s.deployment.name"))
		})
	})

	Context("with invalid input", func() {
		It("should error", func() {
			success, err := ConsistOfMetricsWithResourceAttributes("k8s.cluster.name").Match([]byte{1, 2, 3})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})
})

var _ = Describe("ConsistOfMetricsWithResourceAttributeValue", Label("metrics"), func() {
	Context("with nil input", func() {
		It("should error", func() {
			success, err := ConsistOfMetricsWithResourceAttributeValue("k8s.cluster.name", "not-match").Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with input of invalid type", func() {
		It("should error", func() {
			success, _ := ConsistOfMetricsWithResourceAttributeValue("k8s.cluster.name", "not-match").Match(struct{}{})
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should error", func() {
			success, err := ConsistOfMetricsWithResourceAttributeValue("k8s.cluster.name", "not-match").Match([]byte{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with no attribute matching the expecting attributes", func() {
		It("should fail", func() {
			md := pmetric.NewMetrics()
			rm := md.ResourceMetrics().AppendEmpty()
			attrs := rm.Resource().Attributes()
			attrs.PutStr("k8s.cluster.name", "cluster-01")
			attrs.PutStr("k8s.deployment.name", "nginx")
			metrics := rm.ScopeMetrics().AppendEmpty().Metrics()
			kitmetrics.NewGauge().CopyTo(metrics.AppendEmpty())
			kitmetrics.NewCumulativeSum().CopyTo(metrics.AppendEmpty())

			Expect(mustMarshalMetrics(md)).ShouldNot(ConsistOfMetricsWithResourceAttributeValue("k8s.container.name", "not-match"))
		})
	})

	Context("with some attribute matching the expecting attributes", func() {
		It("should fail", func() {
			md := pmetric.NewMetrics()
			rm := md.ResourceMetrics().AppendEmpty()
			attrs := rm.Resource().Attributes()
			attrs.PutStr("k8s.cluster.name", "cluster-01")
			attrs.PutStr("k8s.deployment.name", "nginx")
			metrics := rm.ScopeMetrics().AppendEmpty().Metrics()
			kitmetrics.NewGauge().CopyTo(metrics.AppendEmpty())
			kitmetrics.NewCumulativeSum().CopyTo(metrics.AppendEmpty())

			Expect(mustMarshalMetrics(md)).Should(ConsistOfMetricsWithResourceAttributeValue("k8s.deployment.name", "nginx"))
		})
	})

	Context("with invalid input", func() {
		It("should error", func() {
			success, err := ConsistOfMetricsWithResourceAttributeValue("k8s.cluster.name", "not-match").Match([]byte{1, 2, 3})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})
})

var _ = Describe("ContainMetricsWithNames", Label("metrics"), func() {
	Context("with nil input", func() {
		It("should error", func() {
			success, err := ContainMetricsWithNames("container.cpu.time").Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with input of invalid type", func() {
		It("should error", func() {
			success, err := ContainMetricsWithNames("container.cpu.time").Match(struct{}{})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should fail", func() {
			Expect([]byte{}).ShouldNot(ContainMetricsWithNames("container.cpu.time"))
		})
	})

	Context("with no metric name matching the expecting metric names", func() {
		It("should fail", func() {
			md := pmetric.NewMetrics()
			metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
			gauge1 := kitmetrics.NewGauge(kitmetrics.WithName("container.cpu.time"))
			gauge1.CopyTo(metrics.AppendEmpty())
			gauge2 := kitmetrics.NewGauge(kitmetrics.WithName("container.cpu.utilization"))
			gauge2.CopyTo(metrics.AppendEmpty())

			Expect(mustMarshalMetrics(md)).ShouldNot(ContainMetricsWithNames("container.filesystem.available"))
		})
	})

	Context("with all metric names matching the expecting metric names", func() {
		It("should succeed", func() {
			md := pmetric.NewMetrics()
			metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
			gauge1 := kitmetrics.NewGauge(kitmetrics.WithName("container.cpu.time"))
			gauge1.CopyTo(metrics.AppendEmpty())
			gauge2 := kitmetrics.NewGauge(kitmetrics.WithName("container.cpu.utilization"))
			gauge2.CopyTo(metrics.AppendEmpty())

			Expect(mustMarshalMetrics(md)).Should(ContainMetricsWithNames("container.cpu.time", "container.cpu.utilization"))
		})
	})

	Context("with invalid input", func() {
		It("should error", func() {
			success, err := ContainMetricsWithNames("container.cpu.time").Match([]byte{1, 2, 3})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})
})

func mustMarshalMetrics(md pmetric.Metrics) []byte {
	var marshaler pmetric.JSONMarshaler
	bytes, err := marshaler.MarshalMetrics(md)
	if err != nil {
		panic(err)
	}
	return bytes
}
