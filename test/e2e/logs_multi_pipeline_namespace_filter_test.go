//go:build e2e

package e2e

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogs), Ordered, func() {
	Context("When multiple log pipelines with namespace filter exist", Ordered, func() {
		var (
			mock1Ns                          = suite.IDWithSuffix("1")
			pipelineIncludeNamespaceName     = suite.IDWithSuffix("1")
			backendIncludeNamespaceExportURL string
			backendIncludeNamespaceName      = suite.IDWithSuffix("backend-1")

			mock2Ns                      = suite.IDWithSuffix("2")
			pipelineExcludeNamespaceName = suite.IDWithSuffix("2")
			backend2ExportURL            string
			backendExcludeNamespaceName  = suite.IDWithSuffix("backend-2")
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mock1Ns).K8sObject(),
				kitk8s.NewNamespace(mock2Ns).K8sObject())

			backend1 := backend.New(mock1Ns, backend.SignalTypeLogs, backend.WithName(backendIncludeNamespaceName))

			logProducer1 := loggen.New(mock1Ns)
			backendIncludeNamespaceExportURL = backend1.ExportURL(proxyClient)
			objs = append(objs, backend1.K8sObjects()...)
			objs = append(objs, logProducer1.K8sObject())

			logPipeline1 := testutils.NewLogPipelineBuilder().
				WithName(pipelineIncludeNamespaceName).
				WithIncludeNamespaces(mock1Ns).
				WithHTTPOutput(testutils.HTTPHost(backend1.Host()), testutils.HTTPPort(backend1.Port())).
				Build()
			objs = append(objs, &logPipeline1)

			backend2 := backend.New(mock2Ns, backend.SignalTypeLogs, backend.WithName(backendExcludeNamespaceName))

			logProducer2 := loggen.New(mock2Ns)
			backend2ExportURL = backend2.ExportURL(proxyClient)
			objs = append(objs, backend2.K8sObjects()...)
			objs = append(objs, logProducer2.K8sObject())

			logPipeline2 := testutils.NewLogPipelineBuilder().
				WithName(pipelineExcludeNamespaceName).
				WithExcludeNamespaces(mock1Ns).
				WithHTTPOutput(testutils.HTTPHost(backend2.Host()), testutils.HTTPPort(backend2.Port())).
				Build()
			objs = append(objs, &logPipeline2)

			return objs
		}

		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
				// Wait for LogPipelines to be deleted
				Eventually(func(g Gomega) {
					var pipelines telemetryv1alpha1.LogPipelineList
					g.Expect(k8sClient.List(ctx, &pipelines)).To(Succeed())
					g.Expect(pipelines.Items).To(BeEmpty())
				}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())

			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running logpipelines", func() {
			assert.LogPipelineHealthy(ctx, k8sClient, pipelineIncludeNamespaceName)
			assert.LogPipelineHealthy(ctx, k8sClient, pipelineExcludeNamespaceName)
		})

		It("Should have running log agent", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.FluentBitDaemonSetName)
		})

		It("Should have a log backend running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Namespace: mock1Ns, Name: backendIncludeNamespaceName})
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Namespace: mock2Ns, Name: backendExcludeNamespaceName})
		})

		It("Log pipeline include namespace should have logs from expected namespaces", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendIncludeNamespaceExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(HaveFlatFluentBitLogs(ContainElement(
					HaveNamespace(Equal(mock1Ns)),
				))))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Log pipeline include namespace should have no logs from other namespace in the backend", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(backendIncludeNamespaceExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(HaveFlatFluentBitLogs(Not(ContainElement(
					HaveNamespace(Equal(mock2Ns)),
				)))))
			}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Log pipeline exclude namespace should have logs from other namespaces", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backend2ExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(HaveFlatFluentBitLogs(ContainElement(
					HaveNamespace(Equal(mock2Ns)),
				)),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Log pipeline exclude namespace should have no logs from namespace 1 in the backend", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(backend2ExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(HaveFlatFluentBitLogs(Not(ContainElement(
					HaveNamespace(Equal(mock1Ns)),
				)))))
			}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
