package operator

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

var _ = Describe("Deploying a Telemetry", Ordered, func() {
	const (
		timeout            = time.Second * 10
		interval           = time.Millisecond * 250
		telemetryNamespace = "default"
	)

	Context("When no dependent resources exist", Ordered, func() {
		const telemetryName = "telemetry-1"

		BeforeAll(func() {
			telemetry := &operatorv1alpha1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      telemetryName,
					Namespace: telemetryNamespace,
				},
			}

			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, telemetry)).Should(Succeed())
			})
			Expect(k8sClient.Create(ctx, telemetry)).Should(Succeed())
		})

		It("Should have Telemetry with ready state", func() {
			Eventually(func() (operatorv1alpha1.State, error) {
				lookupKey := types.NamespacedName{
					Name:      telemetryName,
					Namespace: telemetryNamespace,
				}
				var telemetry operatorv1alpha1.Telemetry
				err := k8sClient.Get(ctx, lookupKey, &telemetry)
				if err != nil {
					return "", err
				}

				return telemetry.Status.State, nil
			}, timeout, interval).Should(Equal(operatorv1alpha1.StateReady))
		})
	})

	Context("When a running TracePipeline exists", Ordered, func() {
		const telemetryName = "telemetry-2"
		const traceGRPCEndpoint = "http://traceFoo.kyma-system:4317"
		const traceHTTPEndpoint = "http://traceFoo.kyma-system:4318"

		BeforeAll(func() {
			telemetry := &operatorv1alpha1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      telemetryName,
					Namespace: telemetryNamespace,
				},
			}
			tracePipeline := testutils.NewTracePipelineBuilder().Build()

			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, &tracePipeline)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, telemetry)).Should(Succeed())
			})
			Expect(k8sClient.Create(ctx, telemetry)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &tracePipeline)).Should(Succeed())

			meta.SetStatusCondition(&tracePipeline.Status.Conditions, metav1.Condition{Type: conditions.TypeGatewayHealthy, Status: metav1.ConditionTrue, Reason: conditions.ReasonDeploymentReady})
			meta.SetStatusCondition(&tracePipeline.Status.Conditions, metav1.Condition{Type: conditions.TypeConfigurationGenerated, Status: metav1.ConditionTrue, Reason: conditions.ReasonConfigurationGenerated})
			meta.SetStatusCondition(&tracePipeline.Status.Conditions, metav1.Condition{Type: conditions.TypePending, Status: metav1.ConditionFalse, Reason: conditions.ReasonTraceGatewayDeploymentNotReady})
			meta.SetStatusCondition(&tracePipeline.Status.Conditions, metav1.Condition{Type: conditions.TypeRunning, Status: metav1.ConditionTrue, Reason: conditions.ReasonTraceGatewayDeploymentReady})
			Expect(k8sClient.Status().Update(ctx, &tracePipeline)).Should(Succeed())
		})

		It("Should have Telemetry with ready state", func() {
			Eventually(func() (operatorv1alpha1.State, error) {
				lookupKey := types.NamespacedName{
					Name:      telemetryName,
					Namespace: telemetryNamespace,
				}
				var telemetry operatorv1alpha1.Telemetry
				err := k8sClient.Get(ctx, lookupKey, &telemetry)
				if err != nil {
					return "", err
				}

				return telemetry.Status.State, nil
			}, timeout, interval).Should(Equal(operatorv1alpha1.StateReady))
		})

		It("Should have Telemetry with TracePipeline endpoints", func() {
			Eventually(func(g Gomega) {
				lookupKey := types.NamespacedName{
					Name:      telemetryName,
					Namespace: telemetryNamespace,
				}
				var telemetry operatorv1alpha1.Telemetry
				g.Expect(k8sClient.Get(ctx, lookupKey, &telemetry)).Should(Succeed())
				g.Expect(telemetry.Status.GatewayEndpoints.Traces).ShouldNot(BeNil())
				g.Expect(telemetry.Status.GatewayEndpoints.Traces.GRPC).Should(Equal(traceGRPCEndpoint))
				g.Expect(telemetry.Status.GatewayEndpoints.Traces.HTTP).Should(Equal(traceHTTPEndpoint))
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When a pending TracePipeline exists", Ordered, func() {
		const telemetryName = "telemetry-3"

		BeforeAll(func() {
			telemetry := &operatorv1alpha1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      telemetryName,
					Namespace: telemetryNamespace,
				},
			}
			tracePipeline := testutils.NewTracePipelineBuilder().Build()

			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, &tracePipeline)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, telemetry)).Should(Succeed())
			})
			Expect(k8sClient.Create(ctx, telemetry)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &tracePipeline)).Should(Succeed())

			meta.SetStatusCondition(&tracePipeline.Status.Conditions, metav1.Condition{Type: conditions.TypeGatewayHealthy, Status: metav1.ConditionFalse, Reason: conditions.ReasonDeploymentNotReady})
			meta.SetStatusCondition(&tracePipeline.Status.Conditions, metav1.Condition{Type: conditions.TypeConfigurationGenerated, Status: metav1.ConditionTrue, Reason: conditions.ReasonConfigurationGenerated})
			meta.SetStatusCondition(&tracePipeline.Status.Conditions, metav1.Condition{Type: conditions.TypePending, Status: metav1.ConditionTrue, Reason: conditions.ReasonTraceGatewayDeploymentNotReady})
			Expect(k8sClient.Status().Update(ctx, &tracePipeline)).Should(Succeed())
		})

		It("Should have Telemetry with warning state", func() {
			Eventually(func() (operatorv1alpha1.State, error) {
				lookupKey := types.NamespacedName{
					Name:      telemetryName,
					Namespace: telemetryNamespace,
				}
				var telemetry operatorv1alpha1.Telemetry
				err := k8sClient.Get(ctx, lookupKey, &telemetry)
				if err != nil {
					return "", err
				}
				return telemetry.Status.State, nil
			}, timeout, interval).Should(Equal(operatorv1alpha1.StateWarning))
		})
	})

	Context("When a LogPipeline with Loki output exists", Ordered, func() {
		const (
			telemetryName = "telemetry-4"
			pipelineName  = "pipeline-with-loki-output"
		)

		BeforeAll(func() {
			telemetry := &operatorv1alpha1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      telemetryName,
					Namespace: telemetryNamespace,
				},
			}
			logPipelineWithLokiOutput := &telemetryv1alpha1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name:       pipelineName,
					Generation: 1,
				},
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.Output{
						Loki: &telemetryv1alpha1.LokiOutput{
							URL: telemetryv1alpha1.ValueType{
								Value: "http://logging-loki:3100/loki/api/v1/push",
							},
						},
					}},
			}

			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, logPipelineWithLokiOutput)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, telemetry)).Should(Succeed())
			})
			Expect(k8sClient.Create(ctx, telemetry)).Should(Succeed())
			Expect(k8sClient.Create(ctx, logPipelineWithLokiOutput)).Should(Succeed())

			meta.SetStatusCondition(&logPipelineWithLokiOutput.Status.Conditions, metav1.Condition{Type: conditions.TypeAgentHealthy, Status: metav1.ConditionTrue, Reason: conditions.ReasonDaemonSetReady})
			meta.SetStatusCondition(&logPipelineWithLokiOutput.Status.Conditions, metav1.Condition{Type: conditions.TypeConfigurationGenerated, Status: metav1.ConditionFalse, Reason: conditions.ReasonUnsupportedLokiOutput})
			meta.SetStatusCondition(&logPipelineWithLokiOutput.Status.Conditions, metav1.Condition{Type: conditions.TypePending, Status: metav1.ConditionTrue, Reason: conditions.ReasonUnsupportedLokiOutput})
			Expect(k8sClient.Status().Update(ctx, logPipelineWithLokiOutput)).Should(Succeed())

		})

		It("Should have Telemetry with warning state", func() {
			Eventually(func(g Gomega) {
				lookupKey := types.NamespacedName{
					Name:      telemetryName,
					Namespace: telemetryNamespace,
				}
				var telemetry operatorv1alpha1.Telemetry
				g.Expect(k8sClient.Get(ctx, lookupKey, &telemetry)).Should(Succeed())
				g.Expect(telemetry.Status.State).Should(Equal(operatorv1alpha1.StateWarning))
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When a running MetricPipeline exists", Ordered, func() {
		const telemetryName = "telemetry-5"
		const metricGRPCEndpoint = "http://metricFoo.kyma-system:4317"
		const metricHTTPEndpoint = "http://metricFoo.kyma-system:4318"

		BeforeAll(func() {
			telemetry := &operatorv1alpha1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      telemetryName,
					Namespace: telemetryNamespace,
				},
			}
			runningMetricPipeline := testutils.NewMetricPipelineBuilder().Build()

			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, &runningMetricPipeline)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, telemetry)).Should(Succeed())
			})
			Expect(k8sClient.Create(ctx, &runningMetricPipeline)).Should(Succeed())
			Expect(k8sClient.Create(ctx, telemetry)).Should(Succeed())
		})

		It("Should have Telemetry with ready state", func() {
			Eventually(func() (operatorv1alpha1.State, error) {
				lookupKey := types.NamespacedName{
					Name:      telemetryName,
					Namespace: telemetryNamespace,
				}
				var telemetry operatorv1alpha1.Telemetry
				err := k8sClient.Get(ctx, lookupKey, &telemetry)
				if err != nil {
					return "", err
				}

				return telemetry.Status.State, nil
			}, timeout, interval).Should(Equal(operatorv1alpha1.StateReady))
		})

		It("Should have Telemetry with MetricPipeline endpoints", func() {
			Eventually(func(g Gomega) {
				lookupKey := types.NamespacedName{
					Name:      telemetryName,
					Namespace: telemetryNamespace,
				}
				var telemetry operatorv1alpha1.Telemetry
				g.Expect(k8sClient.Get(ctx, lookupKey, &telemetry)).Should(Succeed())
				g.Expect(telemetry.Status.GatewayEndpoints.Metrics).ShouldNot(BeNil())
				g.Expect(telemetry.Status.GatewayEndpoints.Metrics.GRPC).Should(Equal(metricGRPCEndpoint))
				g.Expect(telemetry.Status.GatewayEndpoints.Metrics.HTTP).Should(Equal(metricHTTPEndpoint))
			}, timeout, interval).Should(Succeed())
		})
	})

})
