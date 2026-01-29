package misc

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers/log/fluentbit"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestOverrides(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelTelemetry, suite.LabelFluentBit)

	const (
		appNameLabelKey = "app.kubernetes.io/name"
	)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		overrides    *corev1.ConfigMap
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit)
	logPipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithExcludeNamespaces().
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
		Build()

	metricPipeline := testutils.NewMetricPipelineBuilder().WithName(pipelineName).Build()
	tracePipeline := testutils.NewTracePipelineBuilder().WithName(pipelineName).Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		&logPipeline,
		&metricPipeline,
		&tracePipeline,
	}

	resources = append(resources, backend.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	// Verify that before overrides we don't have any DEBUG logs
	assert.FluentBitLogPipelineHealthy(t, pipelineName)
	assert.BackendReachable(t, backend)

	assert.BackendDataEventuallyMatches(t, backend,
		fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
			fluentbit.HavePodName(ContainSubstring("telemetry-manager")),
			fluentbit.HaveLevel(Equal("INFO")),
		))),
		assert.WithOptionalDescription("should have logs from the telemetry-manager pod with INFO level"))

	assert.BackendDataConsistentlyMatches(t, backend,
		fluentbit.HaveFlatLogs(Not(ContainElement(SatisfyAll(
			fluentbit.HavePodName(ContainSubstring("telemetry-manager")),
			fluentbit.HaveLevel(Equal("DEBUG")),
			fluentbit.HaveTimestamp(BeTemporally(">=", time.Now().UTC())),
		)))),
		assert.WithOptionalDescription("should NOT have logs from the telemetry-manager pod with DEBUG level"))

	// Verify that after overrides config we have DEBUG logs
	timeBeforeCreatingOverrides := time.Now().UTC().Truncate(time.Second)
	overrides = kitk8sobjects.NewOverrides().WithLogLevel(kitk8sobjects.DEBUG).K8sObject()
	Expect(kitk8s.CreateObjects(t, overrides)).Should(Succeed())

	triggerLogPipelineReconcilation(pipelineName)

	assert.BackendDataEventuallyMatches(t, backend,
		fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
			fluentbit.HavePodName(ContainSubstring("telemetry-manager")),
			fluentbit.HaveLevel(Equal("DEBUG")),
			fluentbit.HaveTimestamp(BeTemporally(">=", timeBeforeCreatingOverrides)),
		))),
		assert.WithOptionalDescription("should have logs from the telemetry-manager pod with DEBUG level"))

	// Verify that Pipeline reconciliation is disabled for all pipelines
	assertPipelineReconciliationDisabled(suite.Ctx, suite.K8sClient, kitkyma.FluentBitConfigMap, appNameLabelKey)
	assertPipelineReconciliationDisabled(suite.Ctx, suite.K8sClient, kitkyma.MetricGatewayConfigMap, appNameLabelKey)
	assertPipelineReconciliationDisabled(suite.Ctx, suite.K8sClient, kitkyma.TraceGatewayConfigMap, appNameLabelKey)
	assertTelemetryReconciliationDisabled(suite.Ctx, suite.K8sClient, names.ValidatingWebhookConfig)

	// Delete the overrides configmap at the end of the test
	Expect(kitk8s.DeleteObjects(overrides)).Should(Succeed())
}

func assertPipelineReconciliationDisabled(ctx context.Context, k8sClient client.Client, configMapNamespacedName types.NamespacedName, labelKey string) {
	var configMap corev1.ConfigMap
	Expect(k8sClient.Get(ctx, configMapNamespacedName, &configMap)).To(Succeed())

	delete(configMap.Labels, labelKey)
	Expect(k8sClient.Update(ctx, &configMap)).To(Succeed())

	// The deleted label should not be restored, since the reconciliation is disabled by the overrides configmap
	Consistently(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, configMapNamespacedName, &configMap)).To(Succeed())
		g.Expect(configMap.Labels[labelKey]).To(BeZero())
	}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(Succeed(), "Pipeline reconciliation should be disabled")
}

func assertTelemetryReconciliationDisabled(ctx context.Context, k8sClient client.Client, webhookName string) {
	key := types.NamespacedName{
		Name: webhookName,
	}

	var validatingWebhookConfiguration admissionregistrationv1.ValidatingWebhookConfiguration
	Expect(k8sClient.Get(ctx, key, &validatingWebhookConfiguration)).To(Succeed())

	validatingWebhookConfiguration.Webhooks[0].ClientConfig.CABundle = []byte{}
	Expect(k8sClient.Update(ctx, &validatingWebhookConfiguration)).To(Succeed())

	// The deleted CA bundle should not be restored, since the reconciliation is disabled by the overrides configmap
	Consistently(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, key, &validatingWebhookConfiguration)).To(Succeed())
		g.Expect(validatingWebhookConfiguration.Webhooks[0].ClientConfig.CABundle).To(BeEmpty())
	}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(Succeed(), "Telemetry reconciliation should be disabled")
}

func triggerLogPipelineReconcilation(pipelineName string) {
	lookupKey := types.NamespacedName{
		Name: pipelineName,
	}

	var logPipeline telemetryv1beta1.LogPipeline

	err := suite.K8sClient.Get(suite.Ctx, lookupKey, &logPipeline)
	Expect(err).ToNot(HaveOccurred())

	if logPipeline.Annotations == nil {
		logPipeline.Annotations = map[string]string{}
	}

	logPipeline.Annotations["test-annotation"] = "test-value"

	// Update the logPipeline to trigger the reconciliation loop, so that new DEBUG logs are generated
	err = suite.K8sClient.Update(suite.Ctx, &logPipeline)
	Expect(err).ToNot(HaveOccurred())
}
