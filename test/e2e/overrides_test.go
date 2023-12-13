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

	telemetry "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitovrr "github.com/kyma-project/telemetry-manager/test/testkit/kyma/overrides"
	kitlog "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/log"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Overrides", Label("logging"), Ordered, func() {
	const (
		mockBackendName = "overrides-receiver"
		mockNs          = "overrides-log-http-output"
		pipelineName    = "http-output-pipeline"
	)
	var telemetryExportURL string
	var overrides *corev1.ConfigMap
	var now time.Time

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeLogs, backend.WithPersistentHostSecret(isOperational()))
		objs = append(objs, mockBackend.K8sObjects()...)
		telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)

		logPipeline := kitlog.NewPipeline(pipelineName).
			WithSystemNamespaces(true).
			WithSecretKeyRef(mockBackend.HostSecretRef()).
			WithHTTPOutput().
			Persistent(isOperational())
		objs = append(objs, logPipeline.K8sObject())

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
		It("Should have a running logpipeline", Label(operationalTest), func() {
			verifiers.LogPipelineShouldBeRunning(ctx, k8sClient, pipelineName)
		})

		It("Should have a log backend running", Label(operationalTest), func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: mockBackendName})
		})

		It("Should have INFO level logs in the backend", Label(operationalTest), func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ContainLd(ContainLogRecord(SatisfyAll(
						WithPodName(ContainSubstring("telemetry-operator")),
						WithLevel(Equal("INFO")),
					))),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should not have any DEBUG level logs in the backend", Label(operationalTest), func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					Not(ContainLd(ContainLogRecord(SatisfyAll(
						WithPodName(ContainSubstring("telemetry-operator")),
						WithLevel(Equal("DEBUG")),
						WithTimestamp(BeTemporally(">=", now)),
					)))),
				))
			}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should add the overrides configmap and modify the log pipeline", Label(operationalTest), func() {
			// Add overrides configmap
			overrides = kitovrr.NewOverrides(kitovrr.DEBUG).K8sObject()
			Expect(kitk8s.CreateObjects(ctx, k8sClient, overrides)).Should(Succeed())

			// Get logPipeline
			lookupKey := types.NamespacedName{
				Name: pipelineName,
			}
			var logPipeline telemetry.LogPipeline
			err := k8sClient.Get(ctx, lookupKey, &logPipeline)
			Expect(err).ToNot(HaveOccurred())

			// Annotate logPipeline
			if logPipeline.ObjectMeta.Annotations == nil {
				logPipeline.ObjectMeta.Annotations = map[string]string{}
			}
			logPipeline.ObjectMeta.Annotations["test-annotation"] = "test-value"

			// Update logPipeline
			err = k8sClient.Update(ctx, &logPipeline)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should have DEBUG level logs in the backend", Label(operationalTest), func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ContainLd(ContainLogRecord(SatisfyAll(
						WithPodName(ContainSubstring("telemetry-operator")),
						WithLevel(Equal("DEBUG")),
						WithTimestamp(BeTemporally(">=", now)),
					))),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
