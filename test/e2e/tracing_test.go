//go:build e2e

package e2e

import (
	"context"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

var _ = Describe("Tracing", func() {
	Context("When no TracePipeline exists", Ordered, func() {
		BeforeAll(func() {
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
			Expect(k8sClient.Create(ctx, tracePipeline)).Should(Succeed())
			Expect(k8sClient.Create(ctx, externalOTLPTraceService)).Should(Succeed())

			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, externalOTLPTraceService)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, tracePipeline)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, externalMockBackendService)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, mockBackendDeployment)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, mockBackendCm)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, mocksNamespace)).Should(Succeed())
			})
		})

		It("Should send some traces", func() {
			shutdown, err := initProvider("localhost:4317")
			Expect(err).ShouldNot(HaveOccurred())
			defer shutdown(context.Background())
			tracer := otel.Tracer("otlp-load-tester")
			for i := 0; i < 100; i++ {
				_, span := tracer.Start(ctx, "root", trace.WithAttributes(commonAttrs...))
				span.End()
			}
		})

		It("Should retrieve trace data", func() {
			Eventually(func(g Gomega) ([]Span, error) {
				data, err := getResponse("http://localhost:8080/spans.json")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(data).NotTo(BeEmpty())
				return getSpans(data)
			}, timeout, interval).Should(BeEmpty())
		})
	})
})

func makeTracePipeline() *telemetryv1alpha1.TracePipeline {
	return &telemetryv1alpha1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: telemetryv1alpha1.TracePipelineSpec{
			Output: telemetryv1alpha1.TracePipelineOutput{
				Otlp: &telemetryv1alpha1.OtlpOutput{
					Endpoint: telemetryv1alpha1.ValueType{Value: "http://trace-receiver.mocks.svc.cluster.local:4317"},
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
					Name:       "http-otlp",
					Protocol:   corev1.ProtocolTCP,
					Port:       4318,
					TargetPort: intstr.FromInt(4318),
					NodePort:   30018,
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
					Name:       "export-http",
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					NodePort:   30080,
					TargetPort: intstr.FromInt(80),
				},
			},
			Selector: labels,
			Type:     corev1.ServiceTypeNodePort,
		},
	}
}
