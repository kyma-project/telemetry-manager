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
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Overrides", Label("telemetry"), Ordered, func() {
	const (
		mockBackendName = "overrides-receiver"
		mockNs          = "overrides-http-output"
		pipelineName    = "overrides-pipeline"
		appNameLabelKey = "app.kubernetes.io/name"
	)
	var telemetryExportURL string
	var overrides *corev1.ConfigMap
	var now time.Time

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeLogs)
		objs = append(objs, mockBackend.K8sObjects()...)
		telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)

		logPipeline := kitk8s.NewLogPipeline(pipelineName).
			WithSystemNamespaces(true).
			WithSecretKeyRef(mockBackend.HostSecretRef()).
			WithHTTPOutput()
		metricPipeline := kitk8s.NewMetricPipeline(pipelineName)
		tracePipeline := kitk8s.NewTracePipeline(pipelineName)
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
			verifiers.WebhookShouldBeHealthy(ctx, k8sClient)
		})
	})

	Context("When a logpipeline with HTTP output exists", Ordered, func() {
		It("Should have a running logpipeline", func() {
			verifiers.LogPipelineShouldBeHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should have a log backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: mockBackendName})
		})

		It("Should have INFO level logs in the backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
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
				resp, err := proxyClient.Get(telemetryExportURL)
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
			overrides = kitk8s.NewOverrides(kitk8s.DEBUG).K8sObject()
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
				resp, err := proxyClient.Get(telemetryExportURL)
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
			verifiers.PipelineReconciliationShouldBeDisabled(ctx, k8sClient, "telemetry-fluent-bit", appNameLabelKey)
		})

		It("Should disable the reconciliation of the metricpipeline", func() {
			verifiers.PipelineReconciliationShouldBeDisabled(ctx, k8sClient, "telemetry-metric-gateway", appNameLabelKey)
		})

		It("Should disable the reconciliation of the tracepipeline", func() {
			verifiers.PipelineReconciliationShouldBeDisabled(ctx, k8sClient, "telemetry-trace-collector", appNameLabelKey)
		})

		It("Should disable the reconciliation of the telemetry CR", func() {
			verifiers.TelemetryReconciliationShouldBeDisabled(ctx, k8sClient, "validation.webhook.telemetry.kyma-project.io", appNameLabelKey)
		})
	})
})
