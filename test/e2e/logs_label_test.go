//go:build e2e

package e2e

import (
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitlog "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/logproducer"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"

	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Logs Drop Labels", Label("logging"), func() {
	const (
		mockNs          = "log-keep-label-mocks"
		mockBackendName = "log-receiver-label"
		logProducerName = "log-producer"
	)
	var telemetryExportURL string

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeLogs)
		mockLogProducer := logproducer.New(logProducerName, mockNs).
			WithAnnotations(map[string]string{"release": "v1.0.0"})
		objs = append(objs, mockBackend.K8sObjects()...)
		objs = append(objs, mockLogProducer.K8sObject(kitk8s.WithLabel("app", "logging-label-test")))
		telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)

		logPipeline := kitlog.NewPipeline("pipeline-label-test").
			WithSecretKeyRef(mockBackend.HostSecretRef()).
			WithHTTPOutput().
			KeepAnnotations(false).
			DropLabels(false)
		objs = append(objs, logPipeline.K8sObject())

		return objs
	}

	Context("When a logpipeline that keeps labels and drops annotations exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a log backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: mockBackendName})
		})

		It("Should have a log producer running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: logProducerName})
		})

		It("Should have logs with labels in the backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainLd(ContainLogRecord(WithKubernetesLabels(HaveKeyWithValue("app", "logging-label-test")))),
				)))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should have no logs with annotations in the backend", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					Not(ContainLd(ContainLogRecord(WithKubernetesAnnotations(Not(BeEmpty()))))),
				)))
			}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
