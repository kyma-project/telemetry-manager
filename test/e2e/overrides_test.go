//go:build e2e

package e2e

import (
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
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

		logPipeline := kitk8s.NewLogPipelineV1Alpha1(pipelineName).
			WithSystemNamespaces(true).
			WithSecretKeyRef(backend.HostSecretRefV1Alpha1()).
			WithHTTPOutput()
		metricPipeline := kitk8s.NewMetricPipelineV1Alpha1(pipelineName)
		tracePipeline := kitk8s.NewTracePipelineV1Alpha1(pipelineName)
		objs = append(objs, logPipeline.K8sObject(), metricPipeline.K8sObject(), tracePipeline.K8sObject())

		return objs
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
			assert.WebhookShouldBeHealthy(ctx, k8sClient)
		})
	})

	Context("When a logpipeline with HTTP output exists", Ordered, func() {
		It("Should have a running logpipeline", func() {
			assert.LogPipelineShouldBeHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should have a log backend running", func() {
			assert.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: backend.DefaultName})
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
			assert.PipelineReconciliationShouldBeDisabled(ctx, k8sClient, "telemetry-fluent-bit", appNameLabelKey)
		})

		It("Should disable the reconciliation of the metricpipeline", func() {
			assert.PipelineReconciliationShouldBeDisabled(ctx, k8sClient, "telemetry-metric-gateway", appNameLabelKey)
		})

		It("Should disable the reconciliation of the tracepipeline", func() {
			assert.PipelineReconciliationShouldBeDisabled(ctx, k8sClient, "telemetry-trace-collector", appNameLabelKey)
		})

		It("Should disable the reconciliation of the telemetry CR", func() {
			assert.TelemetryReconciliationShouldBeDisabled(ctx, k8sClient, "validation.webhook.telemetry.kyma-project.io", appNameLabelKey)
		})
	})
})
