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

		g.Expect(telemetry.Status.Endpoints.Logs).ShouldNot(BeNil())
		g.Expect(telemetry.Status.Endpoints.Logs.GRPC).Should(Equal(logGRPCEndpoint))
		g.Expect(telemetry.Status.Endpoints.Logs.HTTP).Should(Equal(logHTTPEndpoint))

		g.Expect(telemetry.Status.Endpoints.Traces).ShouldNot(BeNil())
		g.Expect(telemetry.Status.Endpoints.Traces.GRPC).Should(Equal(traceGRPCEndpoint))
		g.Expect(telemetry.Status.Endpoints.Traces.HTTP).Should(Equal(traceHTTPEndpoint))

		g.Expect(telemetry.Status.Endpoints.Metrics).ShouldNot(BeNil())
		g.Expect(telemetry.Status.Endpoints.Metrics.GRPC).Should(Equal(metricGRPCEndpoint))
		g.Expect(telemetry.Status.Endpoints.Metrics.HTTP).Should(Equal(metricHTTPEndpoint))
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

		g.Expect(validatingWebhookConfiguration.Webhooks).Should(HaveLen(3))

		assertWebhook(g,
			findWebhook(validatingWebhookConfiguration.Webhooks, "validating-logpipelines.kyma-project.io"),
			"validating-logpipelines.kyma-project.io",
			"/validate-telemetry-kyma-project-io-v1alpha1-logpipeline",
			"logpipelines")

		assertWebhook(g,
			findWebhook(validatingWebhookConfiguration.Webhooks, "validating-metricpipelines.kyma-project.io"),
			"validating-metricpipelines.kyma-project.io",
			"/validate-telemetry-kyma-project-io-v1alpha1-metricpipeline",
			"metricpipelines")

		assertWebhook(g,
			findWebhook(validatingWebhookConfiguration.Webhooks, "validating-tracepipelines.kyma-project.io"),
			"validating-tracepipelines.kyma-project.io",
			"/validate-telemetry-kyma-project-io-v1alpha1-tracepipeline",
			"tracepipelines")
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

func findWebhook(webhooks []admissionregistrationv1.ValidatingWebhook, name string) *admissionregistrationv1.ValidatingWebhook {
	for i := range webhooks {
		if webhooks[i].Name == name {
			return &webhooks[i]
		}
	}

	return nil
}

func assertWebhook(g Gomega, webhook *admissionregistrationv1.ValidatingWebhook, webhookName, servicePath, ruleResource string) {
	g.Expect(webhook).ShouldNot(BeNil(), "webhook %s not found", webhookName)
	g.Expect(webhook.Name).Should(Equal(webhookName))
	g.Expect(webhook.ClientConfig.CABundle).ShouldNot(BeEmpty())
	g.Expect(webhook.ClientConfig.Service.Name).Should(Equal("telemetry-manager-webhook"))
	g.Expect(webhook.ClientConfig.Service.Namespace).Should(Equal(kitkyma.SystemNamespaceName))
	g.Expect(*webhook.ClientConfig.Service.Port).Should(Equal(int32(443)))
	g.Expect(*webhook.ClientConfig.Service.Path).Should(Equal(servicePath))
	g.Expect(webhook.Rules).Should(HaveLen(1))
	g.Expect(webhook.Rules[0].Resources).Should(ContainElement(ruleResource))
	g.Expect(webhook.Rules[0].Operations).Should(ContainElement(admissionregistrationv1.Create))
	g.Expect(webhook.Rules[0].Operations).Should(ContainElement(admissionregistrationv1.Update))
}
