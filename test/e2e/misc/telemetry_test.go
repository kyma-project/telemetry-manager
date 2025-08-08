package misc

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestTelemetry(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelTelemetry)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("trace-backend")
		genNs        = uniquePrefix("gen")

		traceGRPCEndpoint = "http://telemetry-otlp-traces.kyma-system:4317"
		traceHTTPEndpoint = "http://telemetry-otlp-traces.kyma-system:4318"

		metricGRPCEndpoint = "http://telemetry-otlp-metrics.kyma-system:4317"
		metricHTTPEndpoint = "http://telemetry-otlp-metrics.kyma-system:4318"

		logGRPCEndpoint = "http://telemetry-otlp-logs.kyma-system:4317"
		logHTTPEndpoint = "http://telemetry-otlp-logs.kyma-system:4318"
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeTraces)

	tracePipeline := testutils.NewTracePipelineBuilder().WithName(pipelineName).Build()
	metricPipeline := testutils.NewMetricPipelineBuilder().WithName(pipelineName).Build()
	logPipeline := testutils.NewLogPipelineBuilder().WithName(pipelineName).WithOTLPInput(true).WithOTLPOutput().Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&tracePipeline,
		&metricPipeline,
		&logPipeline,
	}

	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	Eventually(func(g Gomega) {
		var telemetry operatorv1alpha1.Telemetry
		g.Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.TelemetryName, &telemetry)).Should(Succeed())

		g.Expect(telemetry.Status.GatewayEndpoints.Metrics).ShouldNot(BeNil())
		g.Expect(telemetry.Status.GatewayEndpoints.Logs.GRPC).Should(Equal(logGRPCEndpoint))
		g.Expect(telemetry.Status.GatewayEndpoints.Logs.HTTP).Should(Equal(logHTTPEndpoint))

		g.Expect(telemetry.Status.GatewayEndpoints.Traces).ShouldNot(BeNil())
		g.Expect(telemetry.Status.GatewayEndpoints.Traces.GRPC).Should(Equal(traceGRPCEndpoint))
		g.Expect(telemetry.Status.GatewayEndpoints.Traces.HTTP).Should(Equal(traceHTTPEndpoint))

		g.Expect(telemetry.Status.GatewayEndpoints.Metrics).ShouldNot(BeNil())
		g.Expect(telemetry.Status.GatewayEndpoints.Metrics.GRPC).Should(Equal(metricGRPCEndpoint))
		g.Expect(telemetry.Status.GatewayEndpoints.Metrics.HTTP).Should(Equal(metricHTTPEndpoint))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())

	assertValidatingWebhookConfiguration()
	assertWebhookCA()
	assertWebhookSecretReconcilation()
}

func TestTelemetryWarning(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelTelemetry)

	var (
		uniquePrefix = unique.Prefix("warning")
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
	)

	misconfiguredTracePipeline := testutils.NewTracePipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(testutils.OTLPEndpointFromSecret("non-existent-secret", kitkyma.DefaultNamespaceName, "endpoint")).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		&misconfiguredTracePipeline,
	}
	t.Logf("pipeline: %s", misconfiguredTracePipeline.Name)
	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	// assert for misconfigured trace pipeline we have correct telemetry state and condition
	Eventually(func(g Gomega) {
		assert.TelemetryHasState(t, operatorv1alpha1.StateWarning)
		assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
			Type:   conditions.TypeTraceComponentsHealthy,
			Status: metav1.ConditionFalse,
			Reason: conditions.ReasonReferencedSecretMissing,
		})
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

// Decide how to execute it as we delete the telemetry CR in the end of the test
func TestTelemetryDeletionBlocking(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelTelemetry)

	var (
		uniquePrefix = unique.Prefix("delete-blocking")
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
	)

	logBackend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)
	logPipeline := testutils.NewLogPipelineBuilder().WithName(pipelineName).Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		&logPipeline,
	}
	resources = append(resources, logBackend.K8sObjects()...)

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(resources...)).Should(MatchError(ContainSubstring("not found")))
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	var telemetry operatorv1alpha1.Telemetry
	Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.TelemetryName, &telemetry)).Should(Succeed())
	Expect(kitk8s.ForceDeleteObjects(t, &telemetry)).Should(Succeed())

	assertTelemetryCRDeletionIsBlocked(pipelineName)
	// Delete the log pipeline to unblock the deletion of Telemetry CR
	Expect(kitk8s.DeleteObjects(&logPipeline)).Should(Succeed())

	Eventually(func(g Gomega) {
		var telemetry operatorv1alpha1.Telemetry
		g.Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.TelemetryName, &telemetry)).ShouldNot(Succeed())
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())

	Eventually(func(g Gomega) {
		var secret corev1.Secret
		g.Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.WebhookCertSecret, &secret)).Should(Succeed())
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).ShouldNot(Succeed())
}

func assertValidatingWebhookConfiguration() {
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
}

func assertWebhookCA() {
	Eventually(func(g Gomega) {
		var secret corev1.Secret
		g.Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.WebhookCertSecret, &secret)).Should(Succeed())
		g.Expect(secret.OwnerReferences).Should(HaveLen(1))
		g.Expect(secret.OwnerReferences[0].Name).Should(Equal("default"))
		g.Expect(secret.OwnerReferences[0].Kind).Should(Equal("Telemetry"))
		g.Expect(secret.Data).Should(HaveKeyWithValue("ca.crt", Not(BeEmpty())))
		g.Expect(secret.Data).Should(HaveKeyWithValue("ca.key", Not(BeEmpty())))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func assertWebhookSecretReconcilation() {
	var secret corev1.Secret
	Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.WebhookCertSecret, &secret)).Should(Succeed())
	// Delete secret
	oldUID := secret.UID
	Expect(suite.K8sClient.Delete(suite.Ctx, &secret)).Should(Succeed())

	Eventually(func(g Gomega) {
		g.Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.WebhookCertSecret, &secret)).Should(Succeed())
		g.Expect(secret.OwnerReferences).Should(HaveLen(1))
		g.Expect(secret.UID).ShouldNot(Equal(oldUID))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func assertTelemetryCRDeletionIsBlocked(logPipelineName string) {
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
}
