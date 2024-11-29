package log

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/plog"
)

var flsOtel = []FlatLogOtel{
	{
		LogRecordBody: "Test first log body",
		ResourceAttributes: map[string]string{
			"k8s.pod.ip":          "10.42.1.76",
			"k8s.deployment.name": "backend",
			"k8s.namespace.name":  "default",
		},
	},
}

var _ = Describe("HaveFlatOtelLogs", func() {
	It("should apply matcher to valid log data", func() {
		td := plog.NewLogs()
		Expect(mustMarshalOtelLogs(td)).Should(HaveFlatOtelLogs(ContainElements()))
	})

	It("should fail when given empty byte slice", func() {
		Expect([]byte{}).Should(HaveFlatOtelLogs(BeEmpty()))
	})

	It("should return error for nil input", func() {
		success, err := HaveFlatOtelLogs(BeEmpty()).Match(nil)
		Expect(err).Should(HaveOccurred())
		Expect(success).Should(BeFalse())
	})

	It("should return error for invalid input type", func() {
		success, err := HaveFlatOtelLogs(BeEmpty()).Match(struct{}{})
		Expect(err).Should(HaveOccurred())
		Expect(success).Should(BeFalse())
	})

	It("should return a FlatLog struct", func() {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		sl := rl.ScopeLogs().AppendEmpty()
		lr := sl.LogRecords().AppendEmpty()

		// set log body
		lr.Body().SetStr("Test first log body")

		// set resource attributes
		attrs := rl.Resource().Attributes()
		attrs.PutStr("k8s.namespace.name", "default")
		attrs.PutStr("k8s.pod.ip", "10.42.1.76")
		attrs.PutStr("k8s.deployment.name", "backend")

		Expect(mustMarshalOtelLogs(ld)).Should(HaveFlatOtelLogs(ContainElements(flsOtel[0])))
	})
})

var _ = Describe("HaveResourceAttributes", func() {
	It("should apply matcher", func() {
		Expect(flsOtel).Should(ContainElement(HaveResourceAttributes(HaveKey("k8s.deployment.name"))))
	})
})

func mustMarshalOtelLogs(ld plog.Logs) []byte {
	var marshaler plog.JSONMarshaler

	bytes, err := marshaler.MarshalLogs(ld)
	if err != nil {
		panic(err)
	}

	return bytes
}
