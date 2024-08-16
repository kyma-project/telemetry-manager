//go:build e2e

package e2e

import (
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogs), Ordered, func() {
	var (
		mockNs           = suite.ID()
		pipelineName     = suite.ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeLogs)
		logProducer := loggen.New(mockNs).WithAnnotations(map[string]string{"release": "v1.0.0"})
		objs = append(objs, backend.K8sObjects()...)
		objs = append(objs, logProducer.K8sObject())
		backendExportURL = backend.ExportURL(proxyClient)

		logPipeline := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithKeepAnnotations(true).
			WithDropLabels(true).
			WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
			Build()
		objs = append(objs, &logPipeline)

		return objs
	}

	Context("Before deploying a logpipeline", func() {
		It("Should have a healthy webhook", func() {
			assert.WebhookHealthy(ctx, k8sClient)
		})
	})

	Context("When a logpipeline that keeps annotations and drops labels exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running logpipeline", func() {
			assert.LogPipelineHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should have running log agent", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.FluentBitDaemonSetName)
		})

		It("Should have a log backend running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: backend.DefaultName})
		})

		It("Should have a log producer running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: loggen.DefaultName})
		})

		It("Should have logs with annotations in the backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(ContainLd(ContainLogRecord(
					WithKubernetesAnnotations(HaveKeyWithValue("release", "v1.0.0")))),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should have no logs with labels in the backend", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(Not(ContainLd(ContainLogRecord(
					WithKubernetesLabels(Not(BeEmpty()))))),
				))
			}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
