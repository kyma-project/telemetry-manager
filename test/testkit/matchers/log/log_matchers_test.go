package log

import (
	"testing"

	. "github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

var fls = []FlatLog{
	{
		LogRecordBody: "Test first log body",
		ResourceAttributes: map[string]string{
			"k8s.pod.ip":          "10.42.1.76",
			"k8s.deployment.name": "backend",
			"k8s.namespace.name":  "default",
		},
		ObservedTimestamp: "1970-01-01 00:00:01.23456789 +0000 UTC",
		Timestamp:         "1970-01-01 00:00:01.23456789 +0000 UTC",
		Attributes:        map[string]string{"foo": "bar"},
	},
}

func TestOtelLogsMatchers_VerifyInput(t *testing.T) {
	RegisterTestingT(t)

	td := plog.NewLogs()
	Expect(mustMarshalOtelLogs(td)).Should(HaveFlatLogs(ContainElements()), "Should apply matcher to valid log data")
	Expect([]byte{}).Should(HaveFlatLogs(BeEmpty()), "Should fail when given empty byte slice")

	resEmpty, err := HaveFlatLogs(BeEmpty()).Match(nil)
	Expect(err).Should(HaveOccurred(), "Should return error for nil input")
	Expect(resEmpty).Should(BeFalse(), "Success should be false for nil input")

	resInvalidInput, err := HaveFlatLogs(BeEmpty()).Match(struct{}{})
	Expect(err).Should(HaveOccurred(), "should return error for invalid input type")
	Expect(resInvalidInput).Should(BeFalse(), "Success should be false for invalid input type")
}

func TestOtelLogs_FlatLogStruct(t *testing.T) {
	RegisterTestingT(t)

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()

	// set log body
	lr.Body().SetStr("Test first log body")

	testValTimestamp := pcommon.Timestamp(1234567890)
	lr.SetObservedTimestamp(testValTimestamp)
	lr.SetTimestamp(testValTimestamp)

	lr.Attributes().PutStr("foo", "bar")

	// set resource attributes
	attrs := rl.Resource().Attributes()
	attrs.PutStr("k8s.namespace.name", "default")
	attrs.PutStr("k8s.pod.ip", "10.42.1.76")
	attrs.PutStr("k8s.deployment.name", "backend")

	Expect(mustMarshalOtelLogs(ld)).Should(HaveFlatLogs(ContainElements(fls[0])), "Should return a FlatLog struct with expected values")
}

func TestOtelLogsMatchers(t *testing.T) {
	RegisterTestingT(t)
	Expect(fls).Should(ContainElement(HaveResourceAttributes(HaveKey("k8s.deployment.name"))), "Should have key in resource attributes")
}

func mustMarshalOtelLogs(ld plog.Logs) []byte {
	var marshaler plog.JSONMarshaler

	bytes, err := marshaler.MarshalLogs(ld)
	if err != nil {
		panic(err)
	}

	return bytes
}
