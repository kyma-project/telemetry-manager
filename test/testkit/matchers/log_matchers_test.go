//go:build e2e

package matchers

import (
	"go.opentelemetry.io/collector/pdata/plog"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConsistOfNumberOfLogs", Label("logging"), func() {
	Context("with nil input", func() {
		It("should error", func() {
			success, err := ConsistOfNumberOfLogs(0).Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should succeed", func() {
			Expect([]byte{}).Should(ConsistOfNumberOfLogs(0))
		})
	})

	Context("with invalid input", func() {
		It("should error", func() {
			success, err := ConsistOfNumberOfLogs(0).Match([]byte{1, 2, 3})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	It("should succeed if JSONL logs consist of the expected number of logs", func() {
		ld := plog.NewLogs()
		logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
		for i := 0; i < 3; i++ {
			logs.AppendEmpty()
		}

		Expect(mustMarshalLogs(ld)).Should(ConsistOfNumberOfLogs(3))
	})

	It("should fail if JSONL logs do not consist of the expected number of logs", func() {
		ld := plog.NewLogs()
		logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
		for i := 0; i < 2; i++ {
			logs.AppendEmpty()
		}

		Expect(mustMarshalLogs(ld)).ShouldNot(ConsistOfNumberOfLogs(3))
	})
})

var _ = Describe("ContainLogs", Label("logging"), func() {
	Context("with nil input", func() {
		It("should not match", func() {
			success, err := ContainLogs(Any()).Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should match", func() {
			success, err := ContainLogs(Any()).Match([]byte{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with invalid input", func() {
		It("should error", func() {
			success, err := ContainLogs(Any()).Match([]byte{1, 2, 3})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	It("should succeed if JSONL logs contain any logs", func() {
		ld := plog.NewLogs()
		logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
		logs.AppendEmpty()

		Expect(mustMarshalLogs(ld)).Should(ContainLogs(Any()))
	})

	It("should succeed if JSONL logs contain logs that satisfy filter functions", func() {
		ld := plog.NewLogs()
		logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
		k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
		k8sAttrs.PutStr("namespace_name", "log-mocks-single-pipeline")

		Expect(mustMarshalLogs(ld)).Should(ContainLogs(WithNamespace("log-mocks-single-pipeline")))
	})

	It("should fail if JSONL logs do not contain logs that satisfy  filter functions", func() {
		ld := plog.NewLogs()
		logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
		k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
		k8sAttrs.PutStr("namespace_name", "log-mocks-single-pipeline")

		Expect(mustMarshalLogs(ld)).ShouldNot(ContainLogs(WithNamespace("unknown")))
	})
})

var _ = Describe("WithNamespace", func() {
	It("should succeed if the log record has the expected namespace attribute", func() {
		ld := plog.NewLogs()
		logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
		k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
		k8sAttrs.PutStr("namespace_name", "log-mocks-single-pipeline")

		Expect(mustMarshalLogs(ld)).Should(ContainLogs(WithNamespace("log-mocks-single-pipeline")))
	})

	It("should fail if the log record does not have the expected namespace attribute", func() {
		ld := plog.NewLogs()
		logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
		k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
		k8sAttrs.PutStr("namespace_name", "log-mocks-single-pipeline")

		Expect(mustMarshalLogs(ld)).ShouldNot(ContainLogs(WithNamespace("unknown")))
	})
})

var _ = Describe("WithPod", func() {
	It("should succeed if the log record has the expected pod attribute", func() {
		ld := plog.NewLogs()
		logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
		k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
		k8sAttrs.PutStr("pod_name", "log-receiver")

		Expect(mustMarshalLogs(ld)).Should(ContainLogs(WithPod("log-receiver")))
	})

	It("should fail if the log record does not have the expected pod attribute", func() {
		ld := plog.NewLogs()
		logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
		k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
		k8sAttrs.PutStr("pod_name", "log-receiver")

		Expect(mustMarshalLogs(ld)).ShouldNot(ContainLogs(WithPod("unknown")))
	})
})

var _ = Describe("WithContainer", func() {
	It("should succeed if the log record has the expected container attribute", func() {
		ld := plog.NewLogs()
		logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
		k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
		k8sAttrs.PutStr("container_name", "fluentd")

		Expect(mustMarshalLogs(ld)).Should(ContainLogs(WithContainer("fluentd")))
	})

	It("should fail if the log record does not have the expected container attribute", func() {
		ld := plog.NewLogs()
		logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
		k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
		k8sAttrs.PutStr("container_name", "fluentd")

		Expect(mustMarshalLogs(ld)).ShouldNot(ContainLogs(WithContainer("unknown")))
	})
})

var _ = Describe("WithAttributeKeyValue", func() {
	It("should succeed if the log record has the expected attribute key/value", func() {
		ld := plog.NewLogs()
		logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
		logs.AppendEmpty().Attributes().PutStr("user", "foo")
		logs.AppendEmpty().Attributes().PutStr("user", "bar")

		Expect(mustMarshalLogs(ld)).Should(ContainLogs(WithAttributeKeyValue("user", "foo")))
	})

	It("should fail if the log record does not have the expected attribute key", func() {
		ld := plog.NewLogs()
		logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
		logs.AppendEmpty().Attributes().PutStr("user", "foo")
		logs.AppendEmpty().Attributes().PutStr("user", "bar")

		Expect(mustMarshalLogs(ld)).ShouldNot(ContainLogs(WithAttributeKeyValue("unknown", "foo")))
	})

	It("should fail if the log record does not have the expected attribute value", func() {
		ld := plog.NewLogs()
		logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
		logs.AppendEmpty().Attributes().PutStr("user", "foo")
		logs.AppendEmpty().Attributes().PutStr("user", "bar")

		Expect(mustMarshalLogs(ld)).ShouldNot(ContainLogs(WithAttributeKeyValue("user", "unknown")))
	})
})

var _ = Describe("WithAttributeKeys", func() {
	It("should succeed if the log record has all expected attribute keys", func() {
		ld := plog.NewLogs()
		logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
		attrs := logs.AppendEmpty().Attributes()
		attrs.PutStr("color", "green")
		attrs.PutStr("user", "john")
		attrs.PutStr("day", "monday")

		Expect(mustMarshalLogs(ld)).Should(ContainLogs(WithAttributeKeys("user", "color")))
	})

	It("should fail if the log record has only some expected attribute keys", func() {
		ld := plog.NewLogs()
		logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
		attrs := logs.AppendEmpty().Attributes()
		attrs.PutStr("color", "green")
		attrs.PutStr("user", "john")

		Expect(mustMarshalLogs(ld)).ShouldNot(ContainLogs(WithAttributeKeys("color", "day")))
	})

	It("should fail if the log record has none expected attribute keys", func() {
		ld := plog.NewLogs()
		logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
		attrs := logs.AppendEmpty().Attributes()
		attrs.PutStr("user", "john")

		Expect(mustMarshalLogs(ld)).ShouldNot(ContainLogs(WithAttributeKeys("color", "day")))
	})
})

var _ = Describe("WithKubernetesLabels", func() {
	It("should succeed if the log record has kubernetes label attributes", func() {
		ld := plog.NewLogs()
		logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
		k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
		k8sAttrs.PutEmptyMap("labels").PutStr("env", "prod")

		logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")

		Expect(mustMarshalLogs(ld)).Should(ContainLogs(WithKubernetesLabels()))
	})

	It("should fail if the log record does not have kubernetes label attributes", func() {
		ld := plog.NewLogs()
		logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
		logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")

		Expect(mustMarshalLogs(ld)).ShouldNot(ContainLogs(WithKubernetesLabels()))
	})
})

var _ = Describe("WithKubernetesAnnotations", func() {
	It("should succeed if the log record has kubernetes annotation attributes", func() {
		ld := plog.NewLogs()
		logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
		k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
		k8sAttrs.PutEmptyMap("annotations").PutStr("prometheus.io/scrape", "true")

		logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")

		Expect(mustMarshalLogs(ld)).Should(ContainLogs(WithKubernetesAnnotations()))
	})

	It("should fail if the log record does not have kubernetes annotation attributes", func() {
		ld := plog.NewLogs()
		logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
		logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")

		Expect(mustMarshalLogs(ld)).ShouldNot(ContainLogs(WithKubernetesAnnotations()))
	})
})

func mustMarshalLogs(ld plog.Logs) []byte {
	var marshaler plog.JSONMarshaler
	bytes, err := marshaler.MarshalLogs(ld)
	if err != nil {
		panic(err)
	}
	return bytes
}
