//go:build e2e

package e2e

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
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogs), Ordered, func() {
	var (
		mockNs            = suite.ID()
		pipeline1Name     = suite.IDWithSuffix("1")
		pipeline2Name     = suite.IDWithSuffix("2")
		backend1Name      = "backend-1"
		backend1ExportURL string
		backend2Name      = "backend-2"
		backend2ExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		logProducer := loggen.New(mockNs).WithUseJSON()
		objs = append(objs, logProducer.K8sObject())

		// logPipeline1 ships logs without original body to backend1
		backend1 := backend.New(mockNs, backend.SignalTypeLogs, backend.WithName(backend1Name))
		backend1ExportURL = backend1.ExportURL(proxyClient)
		objs = append(objs, backend1.K8sObjects()...)
		logPipeline1 := testutils.NewLogPipelineBuilder().
			WithName(pipeline1Name).
			WithIncludeContainers(loggen.DefaultContainerName).
			WithIncludeNamespaces(mockNs).
			WithKeepOriginalBody(false).
			WithHTTPOutput(testutils.HTTPHost(backend1.Host()), testutils.HTTPPort(backend1.Port())).
			Build()
		objs = append(objs, &logPipeline1)

		// logPipeline2 ships logs with original body to backend2 (default behavior)
		backend2 := backend.New(mockNs, backend.SignalTypeLogs, backend.WithName(backend2Name))
		backend2ExportURL = backend2.ExportURL(proxyClient)
		objs = append(objs, backend2.K8sObjects()...)
		logPipeline2 := testutils.NewLogPipelineBuilder().
			WithName(pipeline2Name).
			WithIncludeContainers(loggen.DefaultContainerName).
			WithIncludeNamespaces(mockNs).
			WithHTTPOutput(testutils.HTTPHost(backend2.Host()), testutils.HTTPPort(backend2.Port())).
			Build()
		objs = append(objs, &logPipeline2)

		return objs
	}

	Context("When 2 logpipelines that keep and drop original log body exist", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have running logpipelines", func() {
			assert.LogPipelineHealthy(ctx, k8sClient, pipeline1Name)
			assert.LogPipelineHealthy(ctx, k8sClient, pipeline2Name)
		})

		It("Should have running log agent", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.FluentBitDaemonSetName)
		})

		It("Should have log backends running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: backend1Name})
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: backend2Name})
		})

		It("Should have a log producer running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: loggen.DefaultName})
		})

		It("Should ship logs without original body to backend1", func() {
			// Log generator produces JSON logs and logPipeline1 drops the original body if JSON keys were successfully extracted
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backend1ExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(HaveFlatFluentBitLogs(HaveEach(
					HaveLogBody(BeEmpty()),
				))))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should ship logs with original body to backend2", func() {
			// Log generator produces JSON logs and logPipeline1 keeps the original body if JSON keys were successfully extracted
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backend2ExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(HaveFlatFluentBitLogs(HaveEach(
					HaveLogBody(Not(BeEmpty())),
				))))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
