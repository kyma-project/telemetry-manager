//go:build e2e

package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelTelemetry), Ordered, func() {

	Context("When a TracePipeline exists", Ordered, func() {
		var (
			tracePipelineName = suite.IDWithSuffix("traces-endpoints")
			traceGRPCEndpoint = "http://telemetry-otlp-traces.kyma-system:4317"
			traceHTTPEndpoint = "http://telemetry-otlp-traces.kyma-system:4318"
		)

		BeforeAll(func() {
			tracePipeline := kitk8s.NewTracePipelineV1Alpha1(tracePipelineName).K8sObject()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, tracePipeline)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, tracePipeline)).Should(Succeed())
		})

		It("Should have Telemetry with TracePipeline endpoints", func() {
			Eventually(func(g Gomega) {
				var telemetry operatorv1alpha1.Telemetry
				g.Expect(k8sClient.Get(ctx, kitkyma.TelemetryName, &telemetry)).Should(Succeed())
				g.Expect(telemetry.Status.GatewayEndpoints.Traces).ShouldNot(BeNil())
				g.Expect(telemetry.Status.GatewayEndpoints.Traces.GRPC).Should(Equal(traceGRPCEndpoint))
				g.Expect(telemetry.Status.GatewayEndpoints.Traces.HTTP).Should(Equal(traceHTTPEndpoint))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})
	})

	Context("When a MetricPipeline exists", Ordered, func() {
		var (
			metricPipelineName = suite.IDWithSuffix("metrics-endpoints")
			metricGRPCEndpoint = "http://telemetry-otlp-metrics.kyma-system:4317"
			metricHTTPEndpoint = "http://telemetry-otlp-metrics.kyma-system:4318"
		)

		BeforeAll(func() {
			metricPipeline := kitk8s.NewMetricPipelineV1Alpha1(metricPipelineName).K8sObject()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, metricPipeline)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, metricPipeline)).Should(Succeed())
		})

		It("Should have Telemetry with MetricPipeline endpoints", func() {
			Eventually(func(g Gomega) {
				var telemetry operatorv1alpha1.Telemetry
				g.Expect(k8sClient.Get(ctx, kitkyma.TelemetryName, &telemetry)).Should(Succeed())
				g.Expect(telemetry.Status.GatewayEndpoints.Metrics).ShouldNot(BeNil())
				g.Expect(telemetry.Status.GatewayEndpoints.Metrics.GRPC).Should(Equal(metricGRPCEndpoint))
				g.Expect(telemetry.Status.GatewayEndpoints.Metrics.HTTP).Should(Equal(metricHTTPEndpoint))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})
	})

	Context("When a LogPipeline with Loki output exists", Ordered, func() {
		var logPipelineName = suite.IDWithSuffix("loki-output")

		BeforeAll(func() {
			logPipeline := testutils.NewLogPipelineBuilder().
				WithName(logPipelineName).
				WithLokiOutput().
				Build()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, &logPipeline)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, &logPipeline)).Should(Succeed())
		})

		It("Should have Telemetry with warning state", func() {
			assert.TelemetryHasWarningState(ctx, k8sClient)
			assert.TelemetryHasCondition(ctx, k8sClient, metav1.Condition{
				Type:   "LogComponentsHealthy",
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonUnsupportedLokiOutput,
			})
		})
	})

	Context("When a misconfigured TracePipeline exists", Ordered, func() {
		var (
			tracePipelineName = suite.IDWithSuffix("missing-secret")
			OTLPEndpointRef   = &telemetryv1alpha1.SecretKeyRef{
				Name:      "non-existent-secret",
				Namespace: "default",
				Key:       "endpoint",
			}
		)

		BeforeAll(func() {
			tracePipeline := kitk8s.NewTracePipelineV1Alpha1(tracePipelineName).WithOutputEndpointFromSecret(OTLPEndpointRef).K8sObject()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, tracePipeline)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, tracePipeline)).Should(Succeed())
		})

		It("Should have Telemetry with warning state", func() {
			assert.TelemetryHasWarningState(ctx, k8sClient)
			assert.TelemetryHasCondition(ctx, k8sClient, metav1.Condition{
				Type:   "TraceComponentsHealthy",
				Status: metav1.ConditionFalse,
				Reason: "TracePipeline" + conditions.ReasonReferencedSecretMissing,
			})
		})
	})

	Context("After creating Telemetry resources", Ordered, func() {
		It("Should have ValidatingWebhookConfiguration", func() {
			Eventually(func(g Gomega) {
				var validatingWebhookConfiguration admissionregistrationv1.ValidatingWebhookConfiguration
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: kitkyma.WebhookName}, &validatingWebhookConfiguration)).Should(Succeed())

				g.Expect(validatingWebhookConfiguration.Webhooks).Should(HaveLen(2))

				logPipelineWebhook := validatingWebhookConfiguration.Webhooks[0]
				g.Expect(logPipelineWebhook.Name).Should(Equal("validation.logpipelines.telemetry.kyma-project.io"))
				g.Expect(logPipelineWebhook.ClientConfig.CABundle).ShouldNot(BeEmpty())
				g.Expect(logPipelineWebhook.ClientConfig.Service.Name).Should(Equal("telemetry-manager-webhook"))
				g.Expect(logPipelineWebhook.ClientConfig.Service.Namespace).Should(Equal(kitkyma.SystemNamespaceName))
				g.Expect(*logPipelineWebhook.ClientConfig.Service.Port).Should(Equal(int32(443)))
				g.Expect(*logPipelineWebhook.ClientConfig.Service.Path).Should(Equal("/validate-logpipeline"))
				g.Expect(logPipelineWebhook.Rules).Should(HaveLen(1))
				g.Expect(logPipelineWebhook.Rules[0].Resources).Should(ContainElement("logpipelines"))
				g.Expect(logPipelineWebhook.Rules[0].Operations).Should(ContainElement(admissionregistrationv1.Create))
				g.Expect(logPipelineWebhook.Rules[0].Operations).Should(ContainElement(admissionregistrationv1.Update))

				logParserWebhook := validatingWebhookConfiguration.Webhooks[1]
				g.Expect(logParserWebhook.Name).Should(Equal("validation.logparsers.telemetry.kyma-project.io"))
				g.Expect(logParserWebhook.ClientConfig.CABundle).ShouldNot(BeEmpty())
				g.Expect(logParserWebhook.ClientConfig.Service.Name).Should(Equal("telemetry-manager-webhook"))
				g.Expect(logParserWebhook.ClientConfig.Service.Namespace).Should(Equal(kitkyma.SystemNamespaceName))
				g.Expect(*logParserWebhook.ClientConfig.Service.Port).Should(Equal(int32(443)))
				g.Expect(*logParserWebhook.ClientConfig.Service.Path).Should(Equal("/validate-logparser"))
				g.Expect(logParserWebhook.Rules).Should(HaveLen(1))
				g.Expect(logParserWebhook.Rules[0].Resources).Should(ContainElement("logparsers"))
				g.Expect(logParserWebhook.Rules[0].Operations).Should(ContainElement(admissionregistrationv1.Create))
				g.Expect(logParserWebhook.Rules[0].Operations).Should(ContainElement(admissionregistrationv1.Update))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should have secret with webhook CA bundle", func() {
			Eventually(func(g Gomega) {
				var secret corev1.Secret
				g.Expect(k8sClient.Get(ctx, kitkyma.WebhookCertSecret, &secret)).Should(Succeed())
				g.Expect(secret.OwnerReferences).Should(HaveLen(1))
				g.Expect(secret.OwnerReferences[0].Name).Should(Equal("default"))
				g.Expect(secret.OwnerReferences[0].Kind).Should(Equal("Telemetry"))
				g.Expect(secret.Data).Should(HaveKeyWithValue("ca.crt", Not(BeEmpty())))
				g.Expect(secret.Data).Should(HaveKeyWithValue("ca.key", Not(BeEmpty())))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should reconcile ValidatingWebhookConfiguration", func() {
			testWebhookReconciliation()
		})

		It("Should reconcile CA bundle secret", func() {
			var oldUID types.UID
			By("Deleting secret", func() {
				var secret corev1.Secret
				Expect(k8sClient.Get(ctx, kitkyma.WebhookCertSecret, &secret)).Should(Succeed())
				oldUID = secret.UID
				Expect(k8sClient.Delete(ctx, &secret)).Should(Succeed())
			})

			Eventually(func(g Gomega) {
				var secret corev1.Secret
				g.Expect(k8sClient.Get(ctx, kitkyma.WebhookCertSecret, &secret)).Should(Succeed())
				g.Expect(secret.OwnerReferences).Should(HaveLen(1))
				g.Expect(secret.UID).ShouldNot(Equal(oldUID))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})
	})

	Context("Deleting Telemetry resources", Ordered, func() {
		var (
			logPipelineName = suite.IDWithSuffix("orphaned")
			logPipeline     = testutils.NewLogPipelineBuilder().
					WithName(logPipelineName).
					WithCustomOutput("Name stdout").
					Build()
		)

		BeforeAll(func() {
			Eventually(func(g Gomega) {
				g.Expect(kitk8s.CreateObjects(ctx, k8sClient, &logPipeline)).Should(Succeed())
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		AfterAll(func() {
			// Re-create Telemetry to have ValidatingWebhookConfiguration for remaining tests
			Eventually(func(g Gomega) {
				newTelemetry := []client.Object{kitk8s.NewTelemetry("default", "kyma-system").K8sObject()}
				g.Expect(kitk8s.CreateObjects(ctx, k8sClient, newTelemetry...)).Should(Succeed())
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())

			Eventually(func(g Gomega) {
				var telemetry operatorv1alpha1.Telemetry
				g.Expect(k8sClient.Get(ctx, kitkyma.TelemetryName, &telemetry)).Should(Succeed())
				g.Expect(telemetry.Status.State).Should(Equal(operatorv1alpha1.StateReady))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should have Telemetry resource", func() {
			Eventually(func(g Gomega) {
				var telemetry operatorv1alpha1.Telemetry
				g.Expect(k8sClient.Get(ctx, kitkyma.TelemetryName, &telemetry)).Should(Succeed())
				g.Expect(telemetry.Status.State).Should(Equal(operatorv1alpha1.StateReady))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should reconcile ValidatingWebhookConfiguration if LogPipeline exists", func() {
			testWebhookReconciliation()
		})

		It("Should not delete Telemetry when LogPipeline exists", func() {
			By("Deleting telemetry", func() {
				Expect(kitk8s.ForceDeleteObjects(ctx, k8sClient, telemetryK8sObject)).Should(Succeed())
			})

			Eventually(func(g Gomega) {
				var telemetry operatorv1alpha1.Telemetry
				g.Expect(k8sClient.Get(ctx, kitkyma.TelemetryName, &telemetry)).Should(Succeed())
				g.Expect(telemetry.Finalizers).Should(HaveLen(1))
				g.Expect(telemetry.Finalizers[0]).Should(Equal("telemetry.kyma-project.io/finalizer"))
				g.Expect(telemetry.Status.State).Should(Equal(operatorv1alpha1.StateWarning))
				expectedConditions := map[string]metav1.Condition{
					"LogComponentsHealthy": {
						Status:  "False",
						Reason:  "ResourceBlocksDeletion",
						Message: fmt.Sprintf("The deletion of the module is blocked. To unblock the deletion, delete the following resources: LogPipelines (%s)", logPipelineName),
					},
					"MetricComponentsHealthy": {
						Status:  "True",
						Reason:  "NoPipelineDeployed",
						Message: "No pipelines have been deployed",
					},
					"TraceComponentsHealthy": {
						Status:  "True",
						Reason:  "NoPipelineDeployed",
						Message: "No pipelines have been deployed",
					},
				}
				g.Expect(telemetry.Status.Conditions).Should(HaveLen(3))
				for _, actualCond := range telemetry.Status.Conditions {
					expectedCond := expectedConditions[actualCond.Type]
					g.Expect(expectedCond.Status).Should(Equal(actualCond.Status))
					g.Expect(expectedCond.Reason).Should(Equal(actualCond.Reason))
					g.Expect(expectedCond.Message).Should(Equal(actualCond.Message))
					g.Expect(actualCond.LastTransitionTime).NotTo(BeZero())
				}

			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should delete Telemetry", func() {
			By("Deleting the orphaned LogPipeline", func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, &logPipeline)).Should(Succeed())
			})

			Eventually(func(g Gomega) {
				var telemetry operatorv1alpha1.Telemetry
				g.Expect(k8sClient.Get(ctx, kitkyma.TelemetryName, &telemetry)).ShouldNot(Succeed())
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should not have Webhook and CA bundle", func() {
			Eventually(func(g Gomega) {
				var validatingWebhookConfiguration admissionregistrationv1.ValidatingWebhookConfiguration
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: kitkyma.WebhookName}, &validatingWebhookConfiguration)).Should(Succeed())
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).ShouldNot(Succeed())

			Eventually(func(g Gomega) {
				var secret corev1.Secret
				g.Expect(k8sClient.Get(ctx, kitkyma.WebhookCertSecret, &secret)).Should(Succeed())
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).ShouldNot(Succeed())
		})
	})
})

func testWebhookReconciliation() {
	var oldUID types.UID
	By("Deleting ValidatingWebhookConfiguration", func() {
		var validatingWebhookConfiguration admissionregistrationv1.ValidatingWebhookConfiguration
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: kitkyma.WebhookName}, &validatingWebhookConfiguration)).Should(Succeed())
		oldUID = validatingWebhookConfiguration.UID
		Expect(k8sClient.Delete(ctx, &validatingWebhookConfiguration)).Should(Succeed())
	})

	Eventually(func(g Gomega) {
		var validatingWebhookConfiguration admissionregistrationv1.ValidatingWebhookConfiguration
		g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: kitkyma.WebhookName}, &validatingWebhookConfiguration)).Should(Succeed())
		g.Expect(validatingWebhookConfiguration.UID).ShouldNot(Equal(oldUID))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}
