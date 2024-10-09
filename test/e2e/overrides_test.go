//go:build e2e

package e2e

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
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
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

		backend := backend.New(mockNs, backend.SignalTypeLogs)
		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(proxyClient)

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

	assertPipelineReconciliationDisabled := func(ctx context.Context, k8sClient client.Client, configMapName string, labelKey string) {
		key := types.NamespacedName{
			Name:      configMapName,
			Namespace: kitkyma.SystemNamespaceName,
		}
		var configMap corev1.ConfigMap
		Expect(k8sClient.Get(ctx, key, &configMap)).To(Succeed())

		delete(configMap.ObjectMeta.Labels, labelKey)
		Expect(k8sClient.Update(ctx, &configMap)).To(Succeed())

		// The deleted label should not be restored, since the reconciliation is disabled by the overrides configmap
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, &configMap)).To(Succeed())
			g.Expect(configMap.ObjectMeta.Labels[labelKey]).To(BeZero())
		}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(Succeed())
	}

	assertTelemetryReconciliationDisabled := func(ctx context.Context, k8sClient client.Client, webhookName string, labelKey string) {
		key := types.NamespacedName{
			Name: webhookName,
		}
		var validatingWebhookConfiguration admissionregistrationv1.ValidatingWebhookConfiguration
		Expect(k8sClient.Get(ctx, key, &validatingWebhookConfiguration)).To(Succeed())

		delete(validatingWebhookConfiguration.ObjectMeta.Labels, labelKey)
		Expect(k8sClient.Update(ctx, &validatingWebhookConfiguration)).To(Succeed())

		// The deleted label should not be restored, since the reconciliation is disabled by the overrides configmap
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, &validatingWebhookConfiguration)).To(Succeed())
			g.Expect(validatingWebhookConfiguration.ObjectMeta.Labels[labelKey]).To(BeZero())
		}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(Succeed())
	}

	BeforeAll(func() {
		now = time.Now().UTC()
		k8sObjects := makeResources()
		DeferCleanup(func() {
			if overrides != nil {
				k8sObjects = append(k8sObjects, overrides)
			}

			Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})
		Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
	})

	Context("Before deploying a logpipeline", func() {
		It("Should have a healthy webhook", func() {
			assert.WebhookHealthy(ctx, k8sClient)
		})
	})

	Context("When a logpipeline with HTTP output exists", Ordered, func() {
		It("Should have a running logpipeline", func() {
			assert.LogPipelineHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should have a log backend running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: backend.DefaultName})
		})

		It("Should have INFO level logs in the backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ContainLd(ContainLogRecord(SatisfyAll(
						WithPodName(ContainSubstring("telemetry-manager")),
						WithLevel(Equal("INFO")),
					))),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should not have any DEBUG level logs in the backend", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					Not(ContainLd(ContainLogRecord(SatisfyAll(
						WithPodName(ContainSubstring("telemetry-manager")),
						WithLevel(Equal("DEBUG")),
						WithTimestamp(BeTemporally(">=", now)),
					)))),
				))
			}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should add the overrides configmap and modify the log pipeline", func() {
			overrides = kitk8s.NewOverrides().WithLogLevel(kitk8s.DEBUG).K8sObject()
			Expect(kitk8s.CreateObjects(ctx, k8sClient, overrides)).Should(Succeed())

			lookupKey := types.NamespacedName{
				Name: pipelineName,
			}
			var logPipeline telemetryv1alpha1.LogPipeline
			err := k8sClient.Get(ctx, lookupKey, &logPipeline)
			Expect(err).ToNot(HaveOccurred())

			if logPipeline.ObjectMeta.Annotations == nil {
				logPipeline.ObjectMeta.Annotations = map[string]string{}
			}
			logPipeline.ObjectMeta.Annotations["test-annotation"] = "test-value"

			// Update the logPipeline to trigger the reconciliation loop, so that new DEBUG logs are generated
			err = k8sClient.Update(ctx, &logPipeline)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should have DEBUG level logs in the backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ContainLd(ContainLogRecord(SatisfyAll(
						WithPodName(ContainSubstring("telemetry-manager")),
						WithLevel(Equal("DEBUG")),
						WithTimestamp(BeTemporally(">=", now)),
					))),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})

	Context("When an overrides configmap exists", func() {
		It("Should disable the reconciliation of the logpipeline", func() {
			assertPipelineReconciliationDisabled(ctx, k8sClient, "telemetry-fluent-bit", appNameLabelKey)
		})

		It("Should disable the reconciliation of the metricpipeline", func() {
			assertPipelineReconciliationDisabled(ctx, k8sClient, "telemetry-metric-gateway", appNameLabelKey)
		})

		It("Should disable the reconciliation of the tracepipeline", func() {
			assertPipelineReconciliationDisabled(ctx, k8sClient, "telemetry-trace-gateway", appNameLabelKey)
		})

		It("Should disable the reconciliation of the telemetry CR", func() {
			assertTelemetryReconciliationDisabled(ctx, k8sClient, "validation.webhook.telemetry.kyma-project.io", appNameLabelKey)
		})
	})
})
