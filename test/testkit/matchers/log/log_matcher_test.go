package log

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/plog"
)

var testTime, _ = time.Parse(time.RFC3339, "2023-12-07T09:36:38Z")
var testTime2, _ = time.Parse(time.RFC3339, "2024-11-06T07:25:45Z")

var fls = []FlatLog{
	{
		LogRecordAttributes: map[string]string{
			"level":      "INFO",
			"user":       "foo",
			"timestamp":  testTime.Format(time.RFC3339),
			"kubernetes": `{"annotations":{"app.kubernetes.io/name":"test-annotation"},"container_name":"test-container","labels":{"app.kubernetes.io/istio":"test-label"},"namespace_name":"test-namespace","pod_name":"test-pod"}`,
		},
		Timestamp:                      testTime,
		LogRecordBody:                  "Test first log body",
		Level:                          "INFO",
		PodName:                        "test-pod",
		ContainerName:                  "test-container",
		NamespaceName:                  "test-namespace",
		KubernetesLabelAttributes:      map[string]any{"app.kubernetes.io/istio": "test-label"},
		KubernetesAnnotationAttributes: map[string]any{"app.kubernetes.io/name": "test-annotation"},
	},
	{

		LogRecordAttributes: map[string]string{
			"level":      "DEBUG",
			"user":       "bar",
			"timestamp":  testTime2.Format(time.RFC3339),
			"kubernetes": `{"annotations":{"app.kubernetes.io/name":"test-annotation"}, "container_name":"test-container-2", "label":{"app.kubernetes.io/istio ":"test-label"}, "namespace_name":"test-namespace-2", "pod_name":"test-pod-2"}`,
		},
		Timestamp:                      testTime2,
		LogRecordBody:                  "Test second log body",
		Level:                          "DEBUG",
		PodName:                        "test-pod-2",
		ContainerName:                  "test-container-2",
		NamespaceName:                  "test-namespace-2",
		KubernetesLabelAttributes:      map[string]any{"app.kubernetes.io/istio": "test-label"},
		KubernetesAnnotationAttributes: map[string]any{"app.kubernetes.io/istio-pilot": "test-annotation"},
	},
}

var _ = Describe("HaveFlatLogs", func() {
	It("should apply matcher to transform valid log data", func() {
		ld := plog.NewLogs()
		Expect(mustMarshalLogs(ld)).Should(HaveFlatLogs(ContainElements()))
	})

	It("should fail when given empty byte slice", func() {
		Expect([]byte{}).Should(HaveFlatLogs(BeEmpty()))
	})

	It("should return error for nil input", func() {
		success, err := HaveFlatLogs(BeEmpty()).Match(nil)
		Expect(err).Should(HaveOccurred())
		Expect(success).Should(BeFalse())
	})

	It("should return error for invalid input type", func() {
		success, err := HaveFlatLogs(BeEmpty()).Match(struct{}{})
		Expect(err).Should(HaveOccurred())
		Expect(success).Should(BeFalse())
	})

	It("should return a FlatLog struct", func() {
		ld := plog.NewLogs()

		rl := ld.ResourceLogs().AppendEmpty()
		sl := rl.ScopeLogs().AppendEmpty()
		lr := sl.LogRecords().AppendEmpty()

		lr.Body().SetStr("Test first log body")

		attrs := lr.Attributes()
		attrs.PutStr("level", "INFO")
		attrs.PutStr("user", "foo")
		attrs.PutStr("timestamp", testTime.Format(time.RFC3339))

		k8sAttrs := attrs.PutEmptyMap("kubernetes")

		k8sLabels := k8sAttrs.PutEmptyMap("labels")
		k8sLabels.PutStr("app.kubernetes.io/istio", "test-label")

		k8sAnnotations := k8sAttrs.PutEmptyMap("annotations")
		k8sAnnotations.PutStr("app.kubernetes.io/name", "test-annotation")

		k8sAttrs.PutStr("pod_name", "test-pod")
		k8sAttrs.PutStr("container_name", "test-container")
		k8sAttrs.PutStr("namespace_name", "test-namespace")

		Expect(mustMarshalLogs(ld)).Should(HaveFlatLogs(ContainElement(fls[0])))
	})
})

var _ = Describe("HaveContainerName", func() {
	It("should apply matcher", func() {
		Expect(fls).Should(ContainElement(HaveContainerName(Equal("test-container"))))
	})
})

var _ = Describe("HaveNamespace", func() {
	It("should apply matcher", func() {
		Expect(fls).Should(ContainElement(HaveNamespace(Equal("test-namespace"))))
	})
})

var _ = Describe("HavePodName", func() {
	It("should apply matcher", func() {
		Expect(fls).Should(ContainElement(HavePodName(Equal("test-pod"))))
	})
})

var _ = Describe("HaveLogRecordAttributes", func() {
	It("should apply matcher", func() {
		Expect(fls).Should(ContainElement(HaveLogRecordAttributes(HaveKeyWithValue("user", "foo"))))
	})
})

var _ = Describe("HaveTimestamp", func() {
	It("should apply matcher", func() {
		expectedTime, err := time.Parse(time.RFC3339, "2023-12-07T09:36:38Z")
		Expect(err).ToNot(HaveOccurred())
		Expect(fls).Should(ContainElement(HaveTimestamp(Equal(expectedTime))))
	})

	It("should apply matcher on timestamp after", func() {
		timestampAfter, err := time.Parse(time.RFC3339, "2023-12-08T09:36:38Z")
		Expect(err).ToNot(HaveOccurred())
		Expect(fls).Should(ContainElement(HaveTimestamp(BeTemporally("<", timestampAfter))))
	})

	It("should apply matcher on timestamp before", func() {
		timestampBefore, err := time.Parse(time.RFC3339, "2023-12-05T09:36:38Z")
		Expect(err).ToNot(HaveOccurred())
		Expect(fls).Should(ContainElement(HaveTimestamp(BeTemporally(">", timestampBefore))))
	})
})

var _ = Describe("HaveKubernetesAnnotations", func() {
	It("should apply matcher", func() {
		Expect(fls).Should(ContainElement(HaveKubernetesAnnotations(HaveKey("app.kubernetes.io/name"))))
	})
})

var _ = Describe("HaveKubernetesLabels", func() {
	It("should apply matcher", func() {
		Expect(fls).Should(ContainElement(HaveKubernetesLabels(HaveKey("app.kubernetes.io/istio"))))
	})
})

var _ = Describe("HaveLogBody", func() {
	It("should apply matcher", func() {
		Expect(fls).Should(ContainElement(HaveLogBody(Equal("Test first log body"))))
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
