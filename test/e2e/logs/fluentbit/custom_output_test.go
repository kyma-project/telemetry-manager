//go:build e2e

package fluentbit

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogsFluentBit), Ordered, func() {
	var (
		mockNs           = suite.ID()
		pipelineName     = suite.ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeLogsFluentBit)
		mockLogProducer := loggen.New(mockNs)
		objs = append(objs, backend.K8sObjects()...)
		objs = append(objs, mockLogProducer.K8sObject())
		backendExportURL = backend.ExportURL(suite.ProxyClient)

		customOutputTemplate := fmt.Sprintf(`
	name   http
	port   %d
	host   %s
	format json`, backend.Port(), backend.Host())
		logPipeline := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithCustomOutput(customOutputTemplate).
			Build()
		objs = append(objs, &logPipeline)

		return objs
	}

	Context("When a logpipeline with custom output exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running logpipeline", func() {
			assert.FluentBitLogPipelineHealthy(suite.Ctx, suite.K8sClient, pipelineName)
		})

		It("Should have unsupportedMode set to true", func() {
			assert.LogPipelineUnsupportedMode(suite.Ctx, suite.K8sClient, pipelineName, true)
		})

		It("Should have running log agent", func() {
			assert.DaemonSetReady(suite.Ctx, suite.K8sClient, kitkyma.FluentBitDaemonSetName)
		})

		It("Should have a log backend running", func() {
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Namespace: mockNs, Name: backend.DefaultName})
		})

		It("Should have a log producer running", func() {
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Namespace: mockNs, Name: loggen.DefaultName})
		})

		It("Should have log-producer logs in the backend", func() {
			assert.FluentBitLogsFromPodDelivered(suite.ProxyClient, backendExportURL, loggen.DefaultName)
		})
	})
})
