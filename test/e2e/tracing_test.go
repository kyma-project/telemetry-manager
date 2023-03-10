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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Tracing", func() {
	Context("When no TracePipeline exists", Ordered, func() {
		tracePipeline := &telemetryv1alpha1.TracePipeline{
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

		It("Should successfully create a TracePipeline", func() {
			Expect(k8sClient.Create(ctx, tracePipeline)).Should(Succeed())
		})

		It("Should have a trace collector Deployment", func() {
			Eventually(func() error {
				var deployment appsv1.Deployment
				deploymentKey := types.NamespacedName{
					Name:      "telemetry-trace-collector",
					Namespace: systemNamespace,
				}
				return k8sClient.Get(ctx, deploymentKey, &deployment)
			}, timeout, interval).Should(Succeed())
		})

		It("Should successfully deploy a mock trace receiver", func() {
			Expect(deployMockTraceReceiver(k8sClient)).Should(Succeed())
		})

		It("Should send some traces", func() {
			Expect(deployTraceExternalService(k8sClient)).Should(Succeed())
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
			Eventually(func() (string, error) {
				data, err := getResponse("http://localhost:8080/spans.json")
				return string(data), err
			}, timeout, interval).ShouldNot(BeEmpty())
		})
	})
})
