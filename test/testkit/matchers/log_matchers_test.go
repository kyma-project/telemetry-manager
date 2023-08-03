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

	Context("with matching number of logs", func() {
		It("should succeed", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			for i := 0; i < 28; i++ {
				logs.AppendEmpty()
			}

			Expect(mustMarshalLogs(ld)).Should(ConsistOfNumberOfLogs(28))
		})
	})
})

var _ = Describe("ContainLogs", Label("logging"), func() {
	Context("with nil input", func() {
		It("should not match", func() {
			success, err := ContainLogs().Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should match", func() {
			success, err := ContainLogs().Match([]byte{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with invalid input", func() {
		It("should error", func() {
			success, err := ContainLogs().Match([]byte{1, 2, 3})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with having no logs", func() {
		It("should fail", func() {
			ld := plog.NewLogs()
			ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()

			Expect(mustMarshalLogs(ld)).ShouldNot(ContainLogs())
		})
	})

	Context("with having logs", func() {
		It("should succeed", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			logs.AppendEmpty()

			Expect(mustMarshalLogs(ld)).Should(ContainLogs())
		})
	})
})

var _ = Describe("ContainLogsWithKubernetesAttributes", Label("logging"), func() {
	Context("with nil input", func() {
		It("should not match", func() {
			success, err := ContainLogsWithKubernetesAttributes("mock_namespace", "mock_pod", "mock_container").Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should match", func() {
			success, err := ContainLogsWithKubernetesAttributes("mock_namespace", "mock_pod", "mock_container").Match([]byte{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with invalid input", func() {
		It("should error", func() {
			success, err := ContainLogsWithKubernetesAttributes("mock_namespace", "mock_pod", "mock_container").Match([]byte{1, 2, 3})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with having logs", func() {
		It("should succeed with namespace", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
			k8sAttrs.PutStr("namespace_name", "log-mocks-single-pipeline")

			Expect(mustMarshalLogs(ld)).Should(ContainLogsWithKubernetesAttributes("log-mocks-single-pipeline", "", ""))
		})

		It("should succeed with pod", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
			k8sAttrs.PutStr("namespace_name", "log-mocks-single-pipeline")
			k8sAttrs.PutStr("pod_name", "log-receiver")

			Expect(mustMarshalLogs(ld)).Should(ContainLogsWithKubernetesAttributes("log-mocks-single-pipeline", "log-receiver", ""))
		})

		It("should succeed with container", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
			k8sAttrs.PutStr("namespace_name", "log-mocks-single-pipeline")
			k8sAttrs.PutStr("container_name", "fluentd")

			Expect(mustMarshalLogs(ld)).Should(ContainLogsWithKubernetesAttributes("log-mocks-single-pipeline", "", "fluentd"))
		})

		It("should fail with namespace", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
			k8sAttrs.PutStr("namespace", "log-mocks-single-pipeline")
			k8sAttrs.PutStr("pod", "log-receiver")

			Expect(mustMarshalLogs(ld)).ShouldNot(ContainLogsWithKubernetesAttributes("not-exist", "", ""))
		})

		It("should fail with pod", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
			k8sAttrs.PutStr("namespace", "log-mocks-single-pipeline")
			k8sAttrs.PutStr("pod", "log-receiver")

			Expect(mustMarshalLogs(ld)).ShouldNot(ContainLogsWithKubernetesAttributes("", "not-exist", ""))
		})
	})
})

var _ = Describe("ContainsLogsWithAttribute", Label("logging"), func() {
	Context("with nil input", func() {
		It("should not match", func() {
			success, err := ContainsLogsWithAttribute("mockKey", "mockKey").Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should not match", func() {
			success, err := ContainsLogsWithAttribute("mockKey", "mockKey").Match([]byte{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with invalid input", func() {
		It("should error", func() {
			success, err := ContainsLogsWithAttribute("mockKey", "mockKey").Match([]byte{1, 2, 3})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with having logs", func() {
		It("should succeed with key value", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			logs.AppendEmpty().Attributes().PutStr("user", "foo")
			logs.AppendEmpty().Attributes().PutStr("user", "bar")

			Expect(mustMarshalLogs(ld)).Should(ContainsLogsWithAttribute("user", "foo"))
		})

		It("should fail with value", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			logs.AppendEmpty().Attributes().PutStr("user", "foo")
			logs.AppendEmpty().Attributes().PutStr("user", "bar")

			Expect(mustMarshalLogs(ld)).ShouldNot(ContainsLogsWithAttribute("user", "not-exist"))
		})

		It("should fail with key", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			logs.AppendEmpty().Attributes().PutStr("user", "foo")
			logs.AppendEmpty().Attributes().PutStr("user", "bar")

			Expect(mustMarshalLogs(ld)).ShouldNot(ContainsLogsWithAttribute("key-not-exist", "foo"))
		})
	})
})

var _ = Describe("ContainsLogsWithKubernetesLabels", Label("logging"), func() {
	Context("with nil input", func() {
		It("should not match", func() {
			success, err := ContainsLogsWithKubernetesLabels().Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should not match", func() {
			success, err := ContainsLogsWithKubernetesLabels().Match([]byte{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with invalid input", func() {
		It("should error", func() {
			success, err := ContainsLogsWithKubernetesLabels().Match([]byte{1, 2, 3})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with having some logs with labels", func() {
		It("should succeed", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
			k8sAttrs.PutEmptyMap("labels").PutStr("env", "prod")

			logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")

			Expect(mustMarshalLogs(ld)).Should(ContainsLogsWithKubernetesLabels())
		})
	})

	Context("with having only logs with labels", func() {
		It("should succeed", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
			k8sAttrs.PutEmptyMap("labels").PutStr("env", "prod")

			k8sAttrs = logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
			k8sAttrs.PutEmptyMap("labels").PutStr("version", "1")

			Expect(mustMarshalLogs(ld)).Should(ContainsLogsWithKubernetesLabels())
		})
	})

	Context("with having no logs", func() {
		It("should fail", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")

			Expect(mustMarshalLogs(ld)).ShouldNot(ContainsLogsWithKubernetesLabels())
		})
	})
})

var _ = Describe("ContainsLogsWithKubernetesAnnotations", Label("logging"), func() {
	Context("with nil input", func() {
		It("should not match", func() {
			success, err := ContainsLogsWithKubernetesAnnotations().Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should not match", func() {
			success, err := ContainsLogsWithKubernetesAnnotations().Match([]byte{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with invalid input", func() {
		It("should error", func() {
			success, err := ContainsLogsWithKubernetesAnnotations().Match([]byte{1, 2, 3})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with having some logs with annotations", func() {
		It("should succeed", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
			k8sAttrs.PutEmptyMap("annotations").PutStr("prometheus.io/scrape", "true")

			logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")

			Expect(mustMarshalLogs(ld)).Should(ContainsLogsWithKubernetesAnnotations())
		})
	})

	Context("with having only logs with annotations", func() {
		It("should succeed", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
			k8sAttrs.PutEmptyMap("annotations").PutStr("prometheus.io/scrape", "true")

			k8sAttrs = logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
			k8sAttrs.PutEmptyMap("annotations").PutStr("prometheus.io/scrape", "false")

			Expect(mustMarshalLogs(ld)).Should(ContainsLogsWithKubernetesAnnotations())
		})
	})

	Context("with having no logs", func() {
		It("should fail", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")

			Expect(mustMarshalLogs(ld)).ShouldNot(ContainsLogsWithKubernetesAnnotations())
		})
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
