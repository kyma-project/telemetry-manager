//go:build e2e

package fluentbit

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelLogs), Ordered, func() {
	Context("When multiple log pipelines with namespace filter exist", Ordered, func() {
		var (
			mock1Ns                          = IDWithSuffix("1")
			pipelineIncludeNamespaceName     = IDWithSuffix("1")
			backendIncludeNamespaceExportURL string
			backendIncludeNamespaceName      = IDWithSuffix("backend-1")

			mock2Ns                      = IDWithSuffix("2")
			pipelineExcludeNamespaceName = IDWithSuffix("2")
			backend2ExportURL            string
			backendExcludeNamespaceName  = IDWithSuffix("backend-2")
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mock1Ns).K8sObject(),
				kitk8s.NewNamespace(mock2Ns).K8sObject())

			backend1 := backend.New(mock1Ns, backend.SignalTypeLogs, backend.WithName(backendIncludeNamespaceName))

			logProducer1 := loggen.New(mock1Ns)
			backendIncludeNamespaceExportURL = backend1.ExportURL(ProxyClient)
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
			backend2ExportURL = backend2.ExportURL(ProxyClient)
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
			K8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient, K8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, K8sObjects...)).Should(Succeed())
		})

		It("Should have a running logpipelines", func() {
			assert.LogPipelineHealthy(Ctx, K8sClient, pipelineIncludeNamespaceName)
			assert.LogPipelineHealthy(Ctx, K8sClient, pipelineExcludeNamespaceName)
		})

		It("Should have running log agent", func() {
			assert.DaemonSetReady(Ctx, K8sClient, kitkyma.FluentBitDaemonSetName)
		})

		It("Should have a log backend running", func() {
			assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Namespace: mock1Ns, Name: backendIncludeNamespaceName})
			assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Namespace: mock2Ns, Name: backendExcludeNamespaceName})
		})

		It("Log pipeline include namespace should have logs from expected namespaces", func() {
			Eventually(func(g Gomega) {
				resp, err := ProxyClient.Get(backendIncludeNamespaceExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(HaveFlatFluentBitLogs(ContainElement(
					HaveNamespace(Equal(mock1Ns)),
				))))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Log pipeline include namespace should have no logs from other namespace in the backend", func() {
			Consistently(func(g Gomega) {
				resp, err := ProxyClient.Get(backendIncludeNamespaceExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(HaveFlatFluentBitLogs(Not(ContainElement(
					HaveNamespace(Equal(mock2Ns)),
				)))))
			}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Log pipeline exclude namespace should have logs from other namespaces", func() {
			Eventually(func(g Gomega) {
				resp, err := ProxyClient.Get(backend2ExportURL)
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
				resp, err := ProxyClient.Get(backend2ExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(HaveFlatFluentBitLogs(Not(ContainElement(
					HaveNamespace(Equal(mock1Ns)),
				)))))
			}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
