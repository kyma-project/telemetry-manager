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

	Context("with having some logs", func() {
		It("should succeed", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			logs.AppendEmpty()

			Expect(mustMarshalLogs(ld)).Should(ContainLogs())
		})
	})

	Context("with log filter", func() {
		Context("should succeed with namespace", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
			k8sAttrs.PutStr("namespace_name", "log-mocks-single-pipeline")

			Expect(mustMarshalLogs(ld)).Should(ContainLogs(WithNamespace("log-mocks-single-pipeline")))
		})

		It("should succeed with pod", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
			k8sAttrs.PutStr("namespace_name", "log-mocks-single-pipeline")
			k8sAttrs.PutStr("pod_name", "log-receiver")

			Expect(mustMarshalLogs(ld)).Should(ContainLogs(WithNamespace("log-mocks-single-pipeline"), WithPod("log-receiver")))
		})

		It("should succeed with container", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
			k8sAttrs.PutStr("namespace_name", "log-mocks-single-pipeline")
			k8sAttrs.PutStr("container_name", "fluentd")

			Expect(mustMarshalLogs(ld)).Should(ContainLogs(WithNamespace("log-mocks-single-pipeline"), WithContainer("fluentd")))
		})

		It("should fail with namespace", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
			k8sAttrs.PutStr("namespace", "log-mocks-single-pipeline")
			k8sAttrs.PutStr("pod", "log-receiver")

			Expect(mustMarshalLogs(ld)).ShouldNot(ContainLogs(WithNamespace("not-exist")))
		})

		It("should fail with pod", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
			k8sAttrs.PutStr("namespace", "log-mocks-single-pipeline")
			k8sAttrs.PutStr("pod", "log-receiver")

			Expect(mustMarshalLogs(ld)).ShouldNot(ContainLogs(WithPod("not-exist")))
		})

		It("should succeed with attribute key value", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			logs.AppendEmpty().Attributes().PutStr("user", "foo")
			logs.AppendEmpty().Attributes().PutStr("user", "bar")

			Expect(mustMarshalLogs(ld)).Should(ContainLogs(WithAttributeKeyValue("user", "foo")))
		})

		It("should fail if no matching value", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			logs.AppendEmpty().Attributes().PutStr("user", "foo")
			logs.AppendEmpty().Attributes().PutStr("user", "bar")

			Expect(mustMarshalLogs(ld)).ShouldNot(ContainLogs(WithAttributeKeyValue("user", "value-not-exist")))
		})

		It("should succeed with attribute key", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			logs.AppendEmpty().Attributes().PutStr("user", "foo")
			logs.AppendEmpty().Attributes().PutStr("user", "bar")

			Expect(mustMarshalLogs(ld)).Should(ContainLogs(WithAttributeKeys("user")))
		})

		It("should fail if not matching attribute key", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			logs.AppendEmpty().Attributes().PutStr("user", "foo")
			logs.AppendEmpty().Attributes().PutStr("user", "bar")

			Expect(mustMarshalLogs(ld)).ShouldNot(ContainLogs(WithAttributeKeys("user_does_not_exist")))
		})

		It("should fail if no matching key", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			logs.AppendEmpty().Attributes().PutStr("user", "foo")
			logs.AppendEmpty().Attributes().PutStr("user", "bar")

			Expect(mustMarshalLogs(ld)).ShouldNot(ContainLogs(WithAttributeKeyValue("key-not-exist", "foo")))
		})

		It("should succeed if some logs have labels", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
			k8sAttrs.PutEmptyMap("labels").PutStr("env", "prod")

			logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")

			Expect(mustMarshalLogs(ld)).Should(ContainLogs(WithKubernetesLabels()))
		})

		It("should fail if no logs have labels", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")

			Expect(mustMarshalLogs(ld)).ShouldNot(ContainLogs(WithKubernetesLabels()))
		})

		It("should succeed if some logs have annotations", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			k8sAttrs := logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")
			k8sAttrs.PutEmptyMap("annotations").PutStr("prometheus.io/scrape", "true")

			logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")

			Expect(mustMarshalLogs(ld)).Should(ContainLogs(WithKubernetesAnnotations()))
		})

		It("should fail if no logs have annotations", func() {
			ld := plog.NewLogs()
			logs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			logs.AppendEmpty().Attributes().PutEmptyMap("kubernetes")

			Expect(mustMarshalLogs(ld)).ShouldNot(ContainLogs(WithKubernetesAnnotations()))
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
