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
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelTelemetry), Ordered, func() {
	const (
		appNameLabelKey = "app.kubernetes.io/name"
	)

	var (
		mockNs           = ID()
		pipelineName     = ID()
		backendExportURL string
		overrides        *corev1.ConfigMap
		now              time.Time
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeLogs)
		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(ProxyClient)

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

	assertPipelineReconciliationDisabled := func(Ctx context.Context, K8sClient client.Client, configMapNamespacedName types.NamespacedName, labelKey string) {
		var configMap corev1.ConfigMap
		Expect(K8sClient.Get(Ctx, configMapNamespacedName, &configMap)).To(Succeed())

		delete(configMap.ObjectMeta.Labels, labelKey)
		Expect(K8sClient.Update(Ctx, &configMap)).To(Succeed())

		// The deleted label should not be restored, since the reconciliation is disabled by the overrides configmap
		Consistently(func(g Gomega) {
			g.Expect(K8sClient.Get(Ctx, configMapNamespacedName, &configMap)).To(Succeed())
			g.Expect(configMap.ObjectMeta.Labels[labelKey]).To(BeZero())
		}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(Succeed())
	}

	assertTelemetryReconciliationDisabled := func(Ctx context.Context, K8sClient client.Client, webhookName string) {
		key := types.NamespacedName{
			Name: webhookName,
		}
		var validatingWebhookConfiguration admissionregistrationv1.ValidatingWebhookConfiguration
		Expect(K8sClient.Get(Ctx, key, &validatingWebhookConfiguration)).To(Succeed())

		validatingWebhookConfiguration.Webhooks[0].ClientConfig.CABundle = []byte{}
		Expect(K8sClient.Update(Ctx, &validatingWebhookConfiguration)).To(Succeed())

		// The deleted CA bundle should not be restored, since the reconciliation is disabled by the overrides configmap
		Consistently(func(g Gomega) {
			g.Expect(K8sClient.Get(Ctx, key, &validatingWebhookConfiguration)).To(Succeed())
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

			Expect(kitk8s.DeleteObjects(Ctx, K8sClient, k8sObjects...)).Should(Succeed())
		})
		Expect(kitk8s.CreateObjects(Ctx, K8sClient, k8sObjects...)).Should(Succeed())
	})

	Context("When a logpipeline with HTTP output exists", Ordered, func() {
		It("Should have a running logpipeline", func() {
			assert.LogPipelineHealthy(Ctx, K8sClient, pipelineName)
		})

		It("Should have a log backend running", func() {
			assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Namespace: mockNs, Name: backend.DefaultName})
		})

		It("Should have INFO level logs in the backend", func() {
			Eventually(func(g Gomega) {
				resp, err := ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					HaveFlatFluentBitLogs(ContainElement(SatisfyAll(
						HavePodName(ContainSubstring("telemetry-manager")),
						HaveLevel(Equal("INFO")),
					))),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should not have any DEBUG level logs in the backend", func() {
			Consistently(func(g Gomega) {
				resp, err := ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					HaveFlatFluentBitLogs(Not(ContainElement(SatisfyAll(
						HavePodName(ContainSubstring("telemetry-manager")),
						HaveLevel(Equal("DEBUG")),
						HaveTimestamp(BeTemporally(">=", now)),
					)))),
				))
			}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should add the overrides configmap and modify the log pipeline", func() {
			overrides = kitk8s.NewOverrides().WithLogLevel(kitk8s.DEBUG).K8sObject()
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, overrides)).Should(Succeed())

			lookupKey := types.NamespacedName{
				Name: pipelineName,
			}
			var logPipeline telemetryv1alpha1.LogPipeline
			err := K8sClient.Get(Ctx, lookupKey, &logPipeline)
			Expect(err).ToNot(HaveOccurred())

			if logPipeline.ObjectMeta.Annotations == nil {
				logPipeline.ObjectMeta.Annotations = map[string]string{}
			}
			logPipeline.ObjectMeta.Annotations["test-annotation"] = "test-value"

			// Update the logPipeline to trigger the reconciliation loop, so that new DEBUG logs are generated
			err = K8sClient.Update(Ctx, &logPipeline)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should have DEBUG level logs in the backend", func() {
			Eventually(func(g Gomega) {
				resp, err := ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					HaveFlatFluentBitLogs(ContainElement(SatisfyAll(
						HavePodName(ContainSubstring("telemetry-manager")),
						HaveLevel(Equal("DEBUG")),
						HaveTimestamp(BeTemporally(">=", now)),
					))),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})

	Context("When an overrides configmap exists", Ordered, func() {
		It("Should disable the reconciliation of the logpipeline", func() {
			assertPipelineReconciliationDisabled(Ctx, K8sClient, kitkyma.FluentBitConfigMap, appNameLabelKey)
		})

		It("Should disable the reconciliation of the metricpipeline", func() {
			assertPipelineReconciliationDisabled(Ctx, K8sClient, kitkyma.MetricGatewayConfigMap, appNameLabelKey)
		})

		It("Should disable the reconciliation of the tracepipeline", func() {
			assertPipelineReconciliationDisabled(Ctx, K8sClient, kitkyma.TraceGatewayConfigMap, appNameLabelKey)
		})

		It("Should disable the reconciliation of the telemetry CR", func() {
			assertTelemetryReconciliationDisabled(Ctx, K8sClient, kitkyma.ValidatingWebhookName)
		})
	})
})
