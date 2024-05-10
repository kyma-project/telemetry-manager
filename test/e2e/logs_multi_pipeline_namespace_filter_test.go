//go:build e2e

package e2e

import (
	"k8s.io/apimachinery/pkg/types"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogs), Ordered, func() {
	Context("When multiple log pipelines with namespace filter exist", Ordered, func() {
		var (
			mock1Ns           = suite.IDWithSuffix("1")
			pipeline1Name     = suite.IDWithSuffix("1")
			backend1ExportURL string
			backend1Name      string

			mock2Ns           = suite.IDWithSuffix("2")
			pipeline2Name     = suite.IDWithSuffix("2")
			backend2ExportURL string
			backend2Name      string
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mock1Ns).K8sObject(),
				kitk8s.NewNamespace(mock2Ns).K8sObject())

			backend1 := backend.New(mock1Ns, backend.SignalTypeLogs, backend.WithName(mock1Ns))
			backend1Name = backend1.Name()
			logProducer1 := loggen.New(mock1Ns)
			backend1ExportURL = backend1.ExportURL(proxyClient)
			objs = append(objs, backend1.K8sObjects()...)
			objs = append(objs, logProducer1.K8sObject())

			logPipeline1 := kitk8s.NewLogPipelineV1Alpha1(pipeline1Name).
				WithSecretKeyRef(backend1.HostSecretRefV1Alpha1()).
				WithHTTPOutput().
				WithIncludeNamespaces([]string{mock1Ns})
			objs = append(objs, logPipeline1.K8sObject())

			backend2 := backend.New(mock2Ns, backend.SignalTypeLogs, backend.WithName(mock2Ns))
			backend2Name = backend2.Name()
			logProducer2 := loggen.New(mock2Ns)
			backend2ExportURL = backend2.ExportURL(proxyClient)
			objs = append(objs, backend2.K8sObjects()...)
			objs = append(objs, logProducer2.K8sObject())

			logPipeline2 := kitk8s.NewLogPipelineV1Alpha1(pipeline2Name).
				WithSecretKeyRef(backend2.HostSecretRefV1Alpha1()).
				WithHTTPOutput().
				WithIncludeNamespaces([]string{mock2Ns})
			objs = append(objs, logPipeline2.K8sObject())

			return objs
		}

		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running logpipelines", func() {
			verifiers.LogPipelineShouldBeHealthy(ctx, k8sClient, pipeline1Name)
			verifiers.LogPipelineShouldBeHealthy(ctx, k8sClient, pipeline2Name)
		})

		It("Should have a log backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mock1Ns, Name: backend1Name})
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mock2Ns, Name: backend2Name})
		})

		It("Log pipeline 1 should have logs from expected namespaces", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backend1ExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ContainLd(
						SatisfyAll(
							ContainLogRecord(WithNamespace(Equal(mock1Ns))),
						)),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Log pipeline 1 should have no logs from namespace 2 in the backend", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(backend1ExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(Not(ContainLd(ContainLogRecord(
					WithNamespace(Equal(mock2Ns))))),
				))
			}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Log pipeline 2 should have logs from expected namespaces", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backend2ExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ContainLd(
						SatisfyAll(
							ContainLogRecord(WithNamespace(Equal(mock2Ns))),
						)),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Log pipeline 2 should have no logs from namespace 1 in the backend", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(backend2ExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(Not(ContainLd(ContainLogRecord(
					WithNamespace(Equal(mock1Ns))))),
				))
			}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
