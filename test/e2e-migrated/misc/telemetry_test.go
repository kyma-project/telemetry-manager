package misc

import (
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
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

func TestTelemetry(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelTelemetry)

	var (
		uniquePrefix          = unique.Prefix()
		pipelineName          = uniquePrefix()
		pipelineMisconfigured = uniquePrefix("misconfigured")
		backendNs             = uniquePrefix("backend")
		genNs                 = uniquePrefix("gen")

		traceGRPCEndpoint = "http://telemetry-otlp-traces.kyma-system:4317"
		traceHTTPEndpoint = "http://telemetry-otlp-traces.kyma-system:4318"

		metricGRPCEndpoint = "http://telemetry-otlp-metrics.kyma-system:4317"
		metricHTTPEndpoint = "http://telemetry-otlp-metrics.kyma-system:4318"

		//logGRPCEndpoint = "http://telemetry-otlp-logs.kyma-system:4317"
		//logHTTPEndpoint = "http://telemetry-otlp-logs.kyma-system:4318"
	)

	traceBackend := kitbackend.New(backendNs, kitbackend.SignalTypeTraces)
	tracePipeline := testutils.NewTracePipelineBuilder().WithName(pipelineName).Build()

	metricBackend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)
	metricPipeline := testutils.NewMetricPipelineBuilder().WithName(pipelineName).Build()

	logBackend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)
	logPipeline := testutils.NewLogPipelineBuilder().WithName(pipelineName).Build()

	misconfiguredTracePipeline := testutils.NewTracePipelineBuilder().
		WithName(pipelineMisconfigured).
		WithOTLPOutput(testutils.OTLPEndpointFromSecret("non-existent-secret", kitkyma.DefaultNamespaceName, "endpoint")).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&tracePipeline,
		&metricPipeline,
		&logPipeline,
		&misconfiguredTracePipeline,
	}

	resources = append(resources, traceBackend.K8sObjects()...)
	resources = append(resources, metricBackend.K8sObjects()...)
	resources = append(resources, logBackend.K8sObjects()...)

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	Eventually(func(g Gomega) {
		var telemetry operatorv1alpha1.Telemetry
		g.Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.TelemetryName, &telemetry)).Should(Succeed())

		g.Expect(telemetry.Status.GatewayEndpoints.Traces).ShouldNot(BeNil())
		g.Expect(telemetry.Status.GatewayEndpoints.Traces.GRPC).Should(Equal(traceGRPCEndpoint))
		g.Expect(telemetry.Status.GatewayEndpoints.Traces.HTTP).Should(Equal(traceHTTPEndpoint))

		g.Expect(telemetry.Status.GatewayEndpoints.Metrics).ShouldNot(BeNil())
		g.Expect(telemetry.Status.GatewayEndpoints.Metrics.GRPC).Should(Equal(metricGRPCEndpoint))
		g.Expect(telemetry.Status.GatewayEndpoints.Metrics.HTTP).Should(Equal(metricHTTPEndpoint))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())

	// assert for misconfigured trace pipeline we have correct telemetry state and condition
	assert.TelemetryHasState(t, operatorv1alpha1.StateWarning)
	assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
		Type:   conditions.TypeTraceComponentsHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonReferencedSecretMissing,
	})

	assertValidatingWebhookConfiguration()
	assertWebhookCA()
	assertWebhookSecretReconcilation()
}

func TestTelemetryDeletionBlocking(t *testing.T) {

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
