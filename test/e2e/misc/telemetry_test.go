//go:build e2e

package misc

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
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
			tracePipeline := testutils.NewTracePipelineBuilder().WithName(tracePipelineName).Build()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(&tracePipeline)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(GinkgoT(), &tracePipeline)).Should(Succeed())
		})

		It("Should have Telemetry with TracePipeline endpoints", func() {
			Eventually(func(g Gomega) {
				var telemetry operatorv1alpha1.Telemetry
				g.Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.TelemetryName, &telemetry)).Should(Succeed())
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
			metricPipeline := testutils.NewMetricPipelineBuilder().WithName(metricPipelineName).Build()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(&metricPipeline)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(GinkgoT(), &metricPipeline)).Should(Succeed())
		})

		It("Should have Telemetry with MetricPipeline endpoints", func() {
			Eventually(func(g Gomega) {
				var telemetry operatorv1alpha1.Telemetry
				g.Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.TelemetryName, &telemetry)).Should(Succeed())
				g.Expect(telemetry.Status.GatewayEndpoints.Metrics).ShouldNot(BeNil())
				g.Expect(telemetry.Status.GatewayEndpoints.Metrics.GRPC).Should(Equal(metricGRPCEndpoint))
				g.Expect(telemetry.Status.GatewayEndpoints.Metrics.HTTP).Should(Equal(metricHTTPEndpoint))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})
	})

	Context("When a misconfigured TracePipeline exists", Ordered, func() {
		var (
			tracePipelineName = suite.IDWithSuffix("missing-secret")
		)

		BeforeAll(func() {
			tracePipeline := testutils.NewTracePipelineBuilder().
				WithName(tracePipelineName).
				WithOTLPOutput(testutils.OTLPEndpointFromSecret("non-existent-secret", kitkyma.DefaultNamespaceName, "endpoint")).
				Build()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(&tracePipeline)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(GinkgoT(), &tracePipeline)).Should(Succeed())
		})

		It("Should have Telemetry with warning state", func() {
			assert.TelemetryHasState(GinkgoT(), operatorv1alpha1.StateWarning)
			assert.TelemetryHasCondition(GinkgoT(), suite.K8sClient, metav1.Condition{
				Type:   conditions.TypeTraceComponentsHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonReferencedSecretMissing,
			})
		})
	})

	Context("After creating Telemetry resources", Ordered, func() {
		It("Should have ValidatingWebhookConfiguration", func() {
			Eventually(func(g Gomega) {
				var validatingWebhookConfiguration admissionregistrationv1.ValidatingWebhookConfiguration
				g.Expect(suite.K8sClient.Get(suite.Ctx, client.ObjectKey{Name: kitkyma.ValidatingWebhookName}, &validatingWebhookConfiguration)).Should(Succeed())

				g.Expect(validatingWebhookConfiguration.Webhooks).Should(HaveLen(2))

				logPipelineWebhook := validatingWebhookConfiguration.Webhooks[0]
				g.Expect(logPipelineWebhook.Name).Should(Equal("validating-logpipelines.kyma-project.io"))
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
				g.Expect(logParserWebhook.Name).Should(Equal("validating-logparsers.kyma-project.io"))
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
				g.Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.WebhookCertSecret, &secret)).Should(Succeed())
				g.Expect(secret.OwnerReferences).Should(HaveLen(1))
				g.Expect(secret.OwnerReferences[0].Name).Should(Equal("default"))
				g.Expect(secret.OwnerReferences[0].Kind).Should(Equal("Telemetry"))
				g.Expect(secret.Data).Should(HaveKeyWithValue("ca.crt", Not(BeEmpty())))
				g.Expect(secret.Data).Should(HaveKeyWithValue("ca.key", Not(BeEmpty())))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should reconcile CA bundle secret", func() {
			var oldUID types.UID
			By("Deleting secret", func() {
				var secret corev1.Secret
				Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.WebhookCertSecret, &secret)).Should(Succeed())
				oldUID = secret.UID
				Expect(suite.K8sClient.Delete(suite.Ctx, &secret)).Should(Succeed())
			})

			Eventually(func(g Gomega) {
				var secret corev1.Secret
				g.Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.WebhookCertSecret, &secret)).Should(Succeed())
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
				g.Expect(kitk8s.CreateObjects(GinkgoT(), &logPipeline)).Should(Succeed())
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		AfterAll(func() {
			// Recreate Telemetry for remaining tests
			Eventually(func(g Gomega) {
				newTelemetry := []client.Object{kitk8s.NewTelemetry("default", "kyma-system").K8sObject()}
				g.Expect(kitk8s.CreateObjects(GinkgoT(), newTelemetry...)).Should(Succeed())
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())

			Eventually(func(g Gomega) {
				var telemetry operatorv1alpha1.Telemetry
				g.Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.TelemetryName, &telemetry)).Should(Succeed())
				g.Expect(telemetry.Status.State).Should(Equal(operatorv1alpha1.StateReady))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())

			// When the Telemetry CR is deleted, the CA cert secret is also deleted. Upon recreating the Telemetry CR,
			// the CA cert, server cert, and CA bundles in the webhook configurations are regenerated.
			// However, due to this change: https://github.com/kubernetes-sigs/controller-runtime/pull/3020,
			// the HTTPS server may take up to 10 seconds to start using the new cert.
			// This sleep ensures the server has sufficient time to switch to the new cert.
			time.Sleep(15 * time.Second)
		})

		It("Should have Telemetry resource", func() {
			Eventually(func(g Gomega) {
				var telemetry operatorv1alpha1.Telemetry
				g.Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.TelemetryName, &telemetry)).Should(Succeed())
				g.Expect(telemetry.Status.State).Should(Equal(operatorv1alpha1.StateReady))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should not delete Telemetry when LogPipeline exists", func() {
			By("Deleting telemetry", func() {
				var telemetry operatorv1alpha1.Telemetry
				Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.TelemetryName, &telemetry)).Should(Succeed())
				Expect(kitk8s.ForceDeleteObjects(GinkgoT(), &telemetry)).Should(Succeed())
			})

			Eventually(func(g Gomega) {
				var telemetry operatorv1alpha1.Telemetry
				g.Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.TelemetryName, &telemetry)).Should(Succeed())
				g.Expect(telemetry.Finalizers).Should(HaveLen(1))
				g.Expect(telemetry.Finalizers[0]).Should(Equal("telemetry.kyma-project.io/finalizer"))
				g.Expect(telemetry.Status.State).Should(Equal(operatorv1alpha1.StateWarning))
				expectedConditions := map[string]metav1.Condition{
					conditions.TypeLogComponentsHealthy: {
						Status:  "False",
						Reason:  "ResourceBlocksDeletion",
						Message: fmt.Sprintf("The deletion of the module is blocked. To unblock the deletion, delete the following resources: LogPipelines (%s)", logPipelineName),
					},
					conditions.TypeMetricComponentsHealthy: {
						Status:  "True",
						Reason:  "NoPipelineDeployed",
						Message: "No pipelines have been deployed",
					},
					conditions.TypeTraceComponentsHealthy: {
						Status:  "True",
						Reason:  "NoPipelineDeployed",
						Message: "No pipelines have been deployed",
					},
				}
				g.Expect(telemetry.Status.Conditions).Should(HaveLen(3))
				for _, actualCond := range telemetry.Status.Conditions {
					expectedCond := expectedConditions[actualCond.Type]
					g.Expect(expectedCond.Status).Should(Equal(actualCond.Status), "Condition: %+v", actualCond)
					g.Expect(expectedCond.Reason).Should(Equal(actualCond.Reason), "Condition: %+v", actualCond)
					g.Expect(expectedCond.Message).Should(Equal(actualCond.Message), "Condition: %+v", actualCond)
					g.Expect(actualCond.LastTransitionTime).NotTo(BeZero())
				}

			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should delete Telemetry", func() {
			By("Deleting the orphaned LogPipeline", func() {
				Expect(kitk8s.DeleteObjects(&logPipeline)).Should(Succeed())
			})

			Eventually(func(g Gomega) {
				var telemetry operatorv1alpha1.Telemetry
				g.Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.TelemetryName, &telemetry)).ShouldNot(Succeed())
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should not have CA bundle secret", func() {
			Eventually(func(g Gomega) {
				var secret corev1.Secret
				g.Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.WebhookCertSecret, &secret)).Should(Succeed())
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).ShouldNot(Succeed())
		})
	})
})
