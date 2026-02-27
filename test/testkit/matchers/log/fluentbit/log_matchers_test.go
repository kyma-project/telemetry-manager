package fluentbit

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/plog"
)

var testTime = time.Date(2023, 12, 07, 9, 36, 38, 0, time.UTC)

var flsFluentBit = []FlatLog{
	{
		Attributes: map[string]string{
			"level":     "INFO",
			"user":      "foo",
			"timestamp": testTime.Format(time.RFC3339),
		},
		LogBody: "Test first log body",
		KubernetesAttributes: map[string]string{
			"pod_name":       "test-pod",
			"container_name": "test-container",
			"namespace_name": "test-namespace",
		},
		KubernetesLabelAttributes:      map[string]any{"app.kubernetes.io/istio": "test-label"},
		KubernetesAnnotationAttributes: map[string]any{"app.kubernetes.io/name": "test-annotation"},
	},
}

func TestFluentBitMatchers_VerifyInput(t *testing.T) {
	RegisterTestingT(t)

	ld := plog.NewLogs()
	Expect(mustMarshalFluentBitLogs(ld)).Should(HaveFlatLogs(ContainElements()), "Should apply matcher to transform valid log data")

	Expect([]byte{}).Should(HaveFlatLogs(BeEmpty()), "Should fail when given empty byte slice")

	resNil, err := HaveFlatLogs(BeEmpty()).Match(nil)
	Expect(err).Should(HaveOccurred(), "Should return error for nil input")
	Expect(resNil).Should(BeFalse(), "Success should be false for nil input")

	resInvalidInput, err := HaveFlatLogs(BeEmpty()).Match(struct{}{})
	Expect(err).Should(HaveOccurred(), "should return error for invalid input type")
	Expect(resInvalidInput).Should(BeFalse(), "Success should be false for invalid input type")
}

func TestFluentBit_FlatLogStruct(t *testing.T) {
	RegisterTestingT(t)

	ld := plog.NewLogs()

	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()

	lr.Body().SetStr("Test first log body")

	attrs := lr.Attributes()
	attrs.PutStr("level", "INFO")
	attrs.PutStr("user", "foo")
	attrs.PutStr("timestamp", "2023-12-07T09:36:38Z")

	k8sAttrs := attrs.PutEmptyMap("kubernetes")

	k8sLabels := k8sAttrs.PutEmptyMap("labels")
	k8sLabels.PutStr("app.kubernetes.io/istio", "test-label")

	k8sAnnotations := k8sAttrs.PutEmptyMap("annotations")
	k8sAnnotations.PutStr("app.kubernetes.io/name", "test-annotation")

	k8sAttrs.PutStr("pod_name", "test-pod")
	k8sAttrs.PutStr("container_name", "test-container")
	k8sAttrs.PutStr("namespace_name", "test-namespace")

	Expect(mustMarshalFluentBitLogs(ld)).Should(HaveFlatLogs(ContainElement(flsFluentBit[0])), "Should contain required elements")
}

func TestFluentBitMatchers(t *testing.T) {
	RegisterTestingT(t)
	Expect(flsFluentBit).Should(ContainElement(HaveContainerName(Equal("test-container"))), "Container name should match")
	Expect(flsFluentBit).Should(ContainElement(HaveNamespace(Equal("test-namespace"))), "Namespace should match")
	Expect(flsFluentBit).Should(ContainElement(HavePodName(Equal("test-pod"))), "Pod name should match")
	Expect(flsFluentBit).Should(ContainElement(HaveAttributes(HaveKeyWithValue("user", "foo"))), "Should have key 'user' with value 'foo'")

	expectedTime, err := time.Parse(time.RFC3339, "2023-12-07T09:36:38Z")
	Expect(err).ToNot(HaveOccurred())
	Expect(flsFluentBit).Should(ContainElement(HaveTimestamp(Equal(expectedTime))), "Timestamp should match expected value")

	timestampAfter, err := time.Parse(time.RFC3339, "2023-12-08T09:36:38Z")
	Expect(err).ToNot(HaveOccurred())
	Expect(flsFluentBit).Should(ContainElement(HaveTimestamp(BeTemporally("<", timestampAfter))), "Should verify timestamp after")

	timestampBefore, err := time.Parse(time.RFC3339, "2023-12-05T09:36:38Z")
	Expect(err).ToNot(HaveOccurred())
	Expect(flsFluentBit).Should(ContainElement(HaveTimestamp(BeTemporally(">", timestampBefore))), "Should verify timestamp before")

	Expect(flsFluentBit).Should(ContainElement(HaveKubernetesAnnotations(HaveKey("app.kubernetes.io/name"))), "Should have Kubernetes annotation with key 'app.kubernetes.io/name'")
	Expect(flsFluentBit).Should(ContainElement(HaveKubernetesLabels(HaveKey("app.kubernetes.io/istio"))), "Should have Kubernetes label with key 'app.kubernetes.io/istio'")

	Expect(flsFluentBit).Should(ContainElement(HaveLogBody(Equal("Test first log body"))), "Log body should match expected value")
}

func TestFluentBit_DateFormat(t *testing.T) {
	RegisterTestingT(t)

	fl := FlatLog{
		Attributes: map[string]string{
			"date": "2023-12-07T09:36:38.123Z",
		},
	}
	Expect(fl).Should(HaveDateISO8601Format(BeTrue()), "Should return true for valid ISO8601 date format")

	flInvalid := FlatLog{
		Attributes: map[string]string{
			"date": "07-12-2023 09:36:38",
		},
	}
	Expect(flInvalid).Should(HaveDateISO8601Format(BeFalse()), "Should return false for invalid ISO8601 date format")

	flMissingDateAttr := FlatLog{
		Attributes: map[string]string{},
	}
	Expect(flMissingDateAttr).Should(HaveDateISO8601Format(BeFalse()), "Should return false when date attribute is missing")

	flUnixTimestamp := FlatLog{
		Attributes: map[string]string{
			"date": "1744288742.123",
		},
	}
	Expect(flUnixTimestamp).Should(HaveDateISO8601Format(BeFalse()), "Should return false for unix timestamp date format")

	flEmptyDate := FlatLog{
		Attributes: map[string]string{
			"date": "",
		},
	}
	Expect(flEmptyDate).Should(HaveDateISO8601Format(BeFalse()), "Should return false when date attribute is empty")
}

func mustMarshalFluentBitLogs(ld plog.Logs) []byte {
	var marshaler plog.JSONMarshaler

	bytes, err := marshaler.MarshalLogs(ld)
	if err != nil {
		panic(err)
	}

	return bytes
}
