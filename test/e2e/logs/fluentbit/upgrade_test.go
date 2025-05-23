//go:build e2e

package fluentbit

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogsFluentBit), Ordered, func() {
	var (
		mockNs       = suite.ID()
		pipelineName = suite.ID()
		backend      *kitbackend.Backend
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend = kitbackend.New(mockNs, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithPersistentHostSecret(suite.IsUpgrade()))
		logProducer := stdloggen.NewDeployment(mockNs)
		objs = append(objs, backend.K8sObjects()...)
		objs = append(objs, logProducer.K8sObject())
		hostSecretRef := backend.HostSecretRefV1Alpha1()

		pipelineBuilder := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithHTTPOutput(
				testutils.HTTPHostFromSecret(
					hostSecretRef.Name,
					hostSecretRef.Namespace,
					hostSecretRef.Key,
				),
				testutils.HTTPPort(backend.Port()),
			)
		if suite.IsUpgrade() {
			pipelineBuilder.WithLabels(kitk8s.PersistentLabel)
		}
		logPipeline := pipelineBuilder.Build()
		objs = append(objs, &logPipeline)

		return objs
	}

	Context("When a logpipeline with HTTP output exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running pipeline", Label(suite.LabelUpgrade), func() {
			assert.FluentBitLogPipelineHealthy(suite.Ctx, suite.K8sClient, pipelineName)
		})

		It("Should have a log backend running", Label(suite.LabelUpgrade), func() {
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Namespace: mockNs, Name: kitbackend.DefaultName})
		})

		It("Should have a log producer running", Label(suite.LabelUpgrade), func() {
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Namespace: mockNs, Name: stdloggen.DefaultName})
		})

		It("Should have produced logs in the backend", Label(suite.LabelUpgrade), func() {
			assert.FluentBitLogsFromPodDelivered(suite.Ctx, backend, stdloggen.DefaultName)
		})
	})
})
