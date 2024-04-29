//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Logs Basic v1beta1", Label("logs", "v1beta1"), Ordered, func() {
	var (
		mockNs           = suite.ID()
		logProducerName  = suite.ID()
		pipelineName     = suite.ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeLogs)
		logProducer := loggen.New(logProducerName, mockNs)
		objs = append(objs, backend.K8sObjects()...)
		objs = append(objs, logProducer.K8sObject(kitk8s.WithLabel("app", logProducerName)))
		backendExportURL = backend.ExportURL(proxyClient)

		logPipeline := kitk8s.NewLogPipelineV1Beta1(pipelineName).
			WithSecretKeyRef(backend.HostSecretRefV1Beta1()).
			WithHTTPOutput()
		objs = append(objs, logPipeline.K8sObject())

		return objs
	}

	Context("Before deploying a logpipeline", func() {
		It("Should have a healthy webhook", func() {
			verifiers.WebhookShouldBeHealthy(ctx, k8sClient)
		})
	})

	Context("When a logpipeline with HTTP output exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running pipeline", func() {
			verifiers.LogPipelineShouldBeHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should have a pipeline with legacy condition types at the end of the conditions list", func() {
			verifiers.LogPipelineShouldHaveLegacyConditionsAtEnd(ctx, k8sClient, pipelineName)
		})

		It("Should have a log backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: backend.DefaultName})
		})

		It("Should have a log producer running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: logProducerName})
		})

		It("Should have produced logs in the backend", func() {
			verifiers.LogsShouldBeDelivered(proxyClient, logProducerName, backendExportURL)
		})
	})
})
