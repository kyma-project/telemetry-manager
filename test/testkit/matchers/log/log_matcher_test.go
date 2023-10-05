package log

import (
	"go.opentelemetry.io/collector/pdata/plog"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("WithLds", func() {
	It("should apply matcher to valid log data", func() {
		ld := plog.NewLogs()
		Expect(mustMarshalLogs(ld)).Should(WithLds(ContainElements()))
	})

	It("should fail when given empty byte slice", func() {
		Expect([]byte{}).Should(WithLds(BeEmpty()))
	})

	It("should return error for nil input", func() {
		success, err := WithLds(BeEmpty()).Match(nil)
		Expect(err).Should(HaveOccurred())
		Expect(success).Should(BeFalse())
	})

	It("should return error for invalid input type", func() {
		success, err := WithLds(BeEmpty()).Match(struct{}{})
		Expect(err).Should(HaveOccurred())
		Expect(success).Should(BeFalse())
	})
})

var _ = Describe("WithLogRecords", func() {
	It("should apply matcher", func() {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		lrs := rl.ScopeLogs().AppendEmpty().LogRecords()
		lrs.AppendEmpty()
		lrs.AppendEmpty()

		Expect(mustMarshalLogs(ld)).Should(ContainLd(WithLogRecords(HaveLen(2))))
	})
})

var _ = Describe("WithContainerName", func() {
	It("should apply matcher", func() {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		lrs := rl.ScopeLogs().AppendEmpty().LogRecords()
		lr := lrs.AppendEmpty()
		lr.Attributes().PutEmptyMap("kubernetes").PutStr("container_name", "nginx")

		Expect(mustMarshalLogs(ld)).Should(ContainLd(ContainLogRecord(WithContainerName(Equal("nginx")))))
	})
})

var _ = Describe("WithPodName", func() {
	It("should apply matcher", func() {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		lrs := rl.ScopeLogs().AppendEmpty().LogRecords()
		lr := lrs.AppendEmpty()
		lr.Attributes().PutEmptyMap("kubernetes").PutStr("pod_name", "nginx")

		Expect(mustMarshalLogs(ld)).Should(ContainLd(ContainLogRecord(WithPodName(Equal("nginx")))))
	})
})

var _ = Describe("WithKubernetesAnnotations", func() {
	It("should apply matcher", func() {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		lrs := rl.ScopeLogs().AppendEmpty().LogRecords()
		lr := lrs.AppendEmpty()
		lr.Attributes().PutEmptyMap("kubernetes").PutEmptyMap("annotations").PutStr("app.kubernetes.io/name", "nginx")

		Expect(mustMarshalLogs(ld)).Should(ContainLd(ContainLogRecord(WithKubernetesAnnotations(HaveKey("app.kubernetes.io/name")))))
	})
})

var _ = Describe("WithKubernetesLabels", func() {
	It("should apply matcher", func() {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		lrs := rl.ScopeLogs().AppendEmpty().LogRecords()
		lr := lrs.AppendEmpty()
		lr.Attributes().PutEmptyMap("kubernetes").PutEmptyMap("labels").PutStr("env", "prod")

		Expect(mustMarshalLogs(ld)).Should(ContainLd(ContainLogRecord(WithKubernetesLabels(HaveKey("env")))))
	})
})

var _ = Describe("WithLogRecordAttrs", func() {
	It("should apply matcher", func() {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		lrs := rl.ScopeLogs().AppendEmpty().LogRecords()
		lr := lrs.AppendEmpty()
		lr.Attributes().PutStr("foo", "bar")

		Expect(mustMarshalLogs(ld)).Should(ContainLd(ContainLogRecord(WithLogRecordAttrs(HaveKey("foo")))))
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
