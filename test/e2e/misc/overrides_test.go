//go:build e2e

package misc

import (
	"context"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers/log/fluentbit"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelTelemetry), Ordered, func() {
	const (
		appNameLabelKey = "app.kubernetes.io/name"
	)

	var (
		mockNs           = suite.ID()
		pipelineName     = suite.ID()
		backendExportURL string
		overrides        *corev1.ConfigMap
		now              time.Time
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := kitbackend.New(mockNs, kitbackend.SignalTypeLogsFluentBit)
		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(suite.ProxyClient)

		logPipeline := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithSystemNamespaces(true).
			WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
			Build()
		metricPipeline := testutils.NewMetricPipelineBuilder().WithName(pipelineName).Build()
		tracePipeline := testutils.NewTracePipelineBuilder().WithName(pipelineName).Build()
		objs = append(objs, &logPipeline, &metricPipeline, &tracePipeline)

		return objs
	}

	assertPipelineReconciliationDisabled := func(ctx context.Context, k8sClient client.Client, configMapNamespacedName types.NamespacedName, labelKey string) {
		var configMap corev1.ConfigMap
		Expect(k8sClient.Get(ctx, configMapNamespacedName, &configMap)).To(Succeed())

		delete(configMap.Labels, labelKey)
		Expect(k8sClient.Update(ctx, &configMap)).To(Succeed())

		// The deleted label should not be restored, since the reconciliation is disabled by the overrides configmap
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, configMapNamespacedName, &configMap)).To(Succeed())
			g.Expect(configMap.Labels[labelKey]).To(BeZero())
		}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(Succeed())
	}

	assertTelemetryReconciliationDisabled := func(ctx context.Context, k8sClient client.Client, webhookName string) {
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
		}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(Succeed())
	}

	BeforeAll(func() {
		now = time.Now().UTC()
		k8sObjects := makeResources()
		DeferCleanup(func() {
			if overrides != nil {
				k8sObjects = append(k8sObjects, overrides)
			}

			Expect(kitk8s.DeleteObjects(suite.Ctx, k8sObjects...)).Should(Succeed())
		})
		Expect(kitk8s.CreateObjects(suite.Ctx, k8sObjects...)).Should(Succeed())
	})

	Context("When a logpipeline with HTTP output exists", Ordered, func() {
		It("Should have a running logpipeline", func() {
			assert.FluentBitLogPipelineHealthy(suite.Ctx, pipelineName)
		})

		It("Should have a log backend running", func() {
			assert.DeploymentReady(suite.Ctx, types.NamespacedName{Namespace: mockNs, Name: kitbackend.DefaultName})
		})

		It("Should have INFO level logs in the backend", func() {
			Eventually(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
						fluentbit.HavePodName(ContainSubstring("telemetry-manager")),
						fluentbit.HaveLevel(Equal("INFO")),
					))),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should not have any DEBUG level logs in the backend", func() {
			Consistently(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					fluentbit.HaveFlatLogs(Not(ContainElement(SatisfyAll(
						fluentbit.HavePodName(ContainSubstring("telemetry-manager")),
						fluentbit.HaveLevel(Equal("DEBUG")),
						fluentbit.HaveTimestamp(BeTemporally(">=", now)),
					)))),
				))
			}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should add the overrides configmap and modify the log pipeline", func() {
			overrides = kitk8s.NewOverrides().WithLogLevel(kitk8s.DEBUG).K8sObject()
			Expect(kitk8s.CreateObjects(suite.Ctx, overrides)).Should(Succeed())

			lookupKey := types.NamespacedName{
				Name: pipelineName,
			}
			var logPipeline telemetryv1alpha1.LogPipeline
			err := suite.K8sClient.Get(suite.Ctx, lookupKey, &logPipeline)
			Expect(err).ToNot(HaveOccurred())

			if logPipeline.Annotations == nil {
				logPipeline.Annotations = map[string]string{}
			}
			logPipeline.Annotations["test-annotation"] = "test-value"

			// Update the logPipeline to trigger the reconciliation loop, so that new DEBUG logs are generated
			err = suite.K8sClient.Update(suite.Ctx, &logPipeline)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should have DEBUG level logs in the backend", func() {
			Eventually(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
						fluentbit.HavePodName(ContainSubstring("telemetry-manager")),
						fluentbit.HaveLevel(Equal("DEBUG")),
						fluentbit.HaveTimestamp(BeTemporally(">=", now)),
					))),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})

	Context("When an overrides configmap exists", Ordered, func() {
		It("Should disable the reconciliation of the logpipeline", func() {
			assertPipelineReconciliationDisabled(suite.Ctx, suite.K8sClient, kitkyma.FluentBitConfigMap, appNameLabelKey)
		})

		It("Should disable the reconciliation of the metricpipeline", func() {
			assertPipelineReconciliationDisabled(suite.Ctx, suite.K8sClient, kitkyma.MetricGatewayConfigMap, appNameLabelKey)
		})

		It("Should disable the reconciliation of the tracepipeline", func() {
			assertPipelineReconciliationDisabled(suite.Ctx, suite.K8sClient, kitkyma.TraceGatewayConfigMap, appNameLabelKey)
		})

		It("Should disable the reconciliation of the telemetry CR", func() {
			assertTelemetryReconciliationDisabled(suite.Ctx, suite.K8sClient, kitkyma.ValidatingWebhookName)
		})
	})
})
