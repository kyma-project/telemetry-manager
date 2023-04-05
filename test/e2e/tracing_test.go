//go:build e2e

package e2e

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	. "github.com/kyma-project/telemetry-manager/internal/otelmatchers"
)

var _ = Describe("Tracing", func() {
	Context("When a tracepipeline exists", Ordered, func() {
		BeforeAll(func() {
			tracePipelineSecret := makeTracePipelineSecret()
			tracePipeline := makeTracePipeline()
			externalOTLPTraceService := makeExternalOTLPTracesService()
			mocksNamespace := makeMocksNamespace()
			mockBackendCm := makeMockBackendConfigMap()
			mockBackendDeployment := makeMockBackendDeployment()
			externalMockBackendService := makeExternalMockBackendService()

			Expect(k8sClient.Create(ctx, mocksNamespace)).Should(Succeed())
			Expect(k8sClient.Create(ctx, mockBackendCm)).Should(Succeed())
			Expect(k8sClient.Create(ctx, mockBackendDeployment)).Should(Succeed())
			Expect(k8sClient.Create(ctx, externalMockBackendService)).Should(Succeed())
			Expect(k8sClient.Create(ctx, tracePipelineSecret)).Should(Succeed())
			Expect(k8sClient.Create(ctx, tracePipeline)).Should(Succeed())
			Expect(k8sClient.Create(ctx, externalOTLPTraceService)).Should(Succeed())

			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, externalOTLPTraceService)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, tracePipeline)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, tracePipelineSecret)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, externalMockBackendService)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, mockBackendDeployment)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, mockBackendCm)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, mocksNamespace)).Should(Succeed())
			})
		})

		It("Should have a running trace collector deployment", func() {
			Eventually(func(g Gomega) bool {
				var deployment appsv1.Deployment
				key := types.NamespacedName{Name: "telemetry-trace-collector", Namespace: systemNamespace}
				g.Expect(k8sClient.Get(ctx, key, &deployment)).To(Succeed())

				listOptions := client.ListOptions{
					LabelSelector: labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels),
					Namespace:     systemNamespace,
				}
				var pods corev1.PodList
				Expect(k8sClient.List(ctx, &pods, &listOptions)).To(Succeed())
				for _, pod := range pods.Items {
					for _, containerStatus := range pod.Status.ContainerStatuses {
						if containerStatus.State.Running == nil {
							return false
						}
					}
				}

				return true
			}, timeout, interval).Should(BeTrue())
		})

		It("Should be able to get trace collector metrics endpoint", func() {
			Eventually(func(g Gomega) {
				resp, err := http.Get("http://localhost:8888/metrics")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
			}, timeout, interval).Should(Succeed())
		})

		It("Should verify end-to-end trace delivery", func() {
			traceID := newTraceID()
			var spanIDs []pcommon.SpanID
			for i := 0; i < 100; i++ {
				spanIDs = append(spanIDs, newSpanID())
			}
			attrs := pcommon.NewMap()
			attrs.PutStr("attrA", "chocolate")
			attrs.PutStr("attrB", "raspberry")
			attrs.PutStr("attrC", "vanilla")

			traces := makeTraces(traceID, spanIDs, attrs)

			sendTraces(context.Background(), traces, "localhost", 4317)

			attrs.PutBool("bool", true)
			Eventually(func(g Gomega) {
				resp, err := http.Get("http://localhost:9090/spans.json")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ConsistOfSpansWithIDs(spanIDs),
					EachHaveTraceID(traceID),
					EachHaveAttributes(attrs))))
			}, timeout, interval).Should(Succeed())
		})
	})
})

func makeTracePipelineSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "trace-rcv-hostname",
			Namespace: "default",
		},
		Type:       "opaque",
		StringData: map[string]string{"trace-host": "http://trace-receiver.mocks.svc.cluster.local:4317"},
	}
}
func makeTracePipeline() *telemetryv1alpha1.TracePipeline {
	return &telemetryv1alpha1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: telemetryv1alpha1.TracePipelineSpec{
			Output: telemetryv1alpha1.TracePipelineOutput{
				Otlp: &telemetryv1alpha1.OtlpOutput{
					Endpoint: telemetryv1alpha1.ValueType{
						ValueFrom: &telemetryv1alpha1.ValueFromSource{
							SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
								Name:      "trace-rcv-hostname",
								Namespace: "default",
								Key:       "trace-host",
							},
						},
					},
				},
			},
		},
	}
}

func makeExternalOTLPTracesService() *corev1.Service {
	labels := map[string]string{
		"app.kubernetes.io/name": "telemetry-trace-collector",
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "telemetry-otlp-traces-external",
			Namespace: "kyma-system",
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "grpc-otlp",
					Protocol:   corev1.ProtocolTCP,
					Port:       4317,
					TargetPort: intstr.FromInt(4317),
					NodePort:   30017,
				},
				{
					Name:       "http-metrics",
					Protocol:   corev1.ProtocolTCP,
					Port:       8888,
					TargetPort: intstr.FromInt(8888),
					NodePort:   30088,
				},
			},
			Selector: labels,
			Type:     corev1.ServiceTypeNodePort,
		},
	}
}

func makeMocksNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mocks",
		},
	}
}

func makeMockBackendDeployment() *appsv1.Deployment {
	labels := map[string]string{
		"app": "trace-receiver",
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "trace-receiver",
			Namespace: "mocks",
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "otel-collector",
							Image: "otel/opentelemetry-collector-contrib:0.70.0",
							Args:  []string{"--config=/etc/collector/config.yaml"},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: pointer.Int64(101),
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "config", MountPath: "/etc/collector"},
								{Name: "data", MountPath: "/traces"},
							},
						},
						{
							Name:  "web",
							Image: "nginx:1.23.3",
							VolumeMounts: []corev1.VolumeMount{
								{Name: "data", MountPath: "/usr/share/nginx/html"},
							},
						},
					},
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup: pointer.Int64(101),
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: "trace-receiver-config"},
								},
							},
						},
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}
}

func makeMockBackendConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "trace-receiver-config",
			Namespace: "mocks",
		},
		Data: map[string]string{
			"config.yaml": `receivers:
  otlp:
    protocols:
      grpc: {}
      http: {}
exporters:
  file:
    path: /traces/spans.json
  logging:
    loglevel: debug
service:
  pipelines:
    traces:
      receivers:
      - otlp
      exporters:
      - file
      - logging`,
		},
	}
}

func makeExternalMockBackendService() *corev1.Service {
	labels := map[string]string{
		"app": "trace-receiver",
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "trace-receiver",
			Namespace: "mocks",
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "grpc-otlp",
					Protocol:   corev1.ProtocolTCP,
					Port:       4317,
					TargetPort: intstr.FromInt(4317),
				},
				{
					Name:       "http-otlp",
					Protocol:   corev1.ProtocolTCP,
					Port:       4318,
					TargetPort: intstr.FromInt(4318),
				},
				{
					Name:       "http-web",
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					NodePort:   30090,
					TargetPort: intstr.FromInt(80),
				},
			},
			Selector: labels,
			Type:     corev1.ServiceTypeNodePort,
		},
	}
}

func newSpanID() pcommon.SpanID {
	var rngSeed int64
	_ = binary.Read(crand.Reader, binary.LittleEndian, &rngSeed)
	randSource := rand.New(rand.NewSource(rngSeed))
	sid := pcommon.SpanID{}
	_, _ = randSource.Read(sid[:])
	return sid
}

func newTraceID() pcommon.TraceID {
	var rngSeed int64
	_ = binary.Read(crand.Reader, binary.LittleEndian, &rngSeed)
	randSource := rand.New(rand.NewSource(rngSeed))
	tid := pcommon.TraceID{}
	_, _ = randSource.Read(tid[:])
	return tid
}

func makeTraces(traceID pcommon.TraceID, spanIDs []pcommon.SpanID, attributes pcommon.Map) ptrace.Traces {
	traces := ptrace.NewTraces()

	spans := traces.ResourceSpans().
		AppendEmpty().
		ScopeSpans().
		AppendEmpty().
		Spans()

	for _, spanID := range spanIDs {
		span := spans.AppendEmpty()
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		span.SetSpanID(spanID)
		span.SetTraceID(traceID)
		attributes.CopyTo(span.Attributes())
	}

	return traces
}

func sendTraces(ctx context.Context, traces ptrace.Traces, host string, port int) {
	sender := testbed.NewOTLPTraceDataSender(host, port)
	Expect(sender.Start()).Should(Succeed())
	Expect(sender.ConsumeTraces(ctx, traces)).Should(Succeed())
	sender.Flush()
}
