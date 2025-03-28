//go:build e2e

package logs

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogs, suite.LabelExperimental), Ordered, func() {
	var (
		mockNs                = suite.ID()
		pipelineNameNoInput   = suite.ID() + "-no-input"
		pipelineNameWithInput = suite.ID() + "-with-input"
	)

	var logPipelineWithInput telemetryv1alpha1.LogPipeline

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeLogs)
		objs = append(objs, backend.K8sObjects()...)

		logPipelineNoInput := testutils.NewLogPipelineBuilder().
			WithName(pipelineNameNoInput).
			WithApplicationInput(false).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()
		objs = append(objs, &logPipelineNoInput)
		logPipelineWithInput = testutils.NewLogPipelineBuilder().
			WithName(pipelineNameWithInput).
			WithApplicationInput(true).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()

		return objs
	}

	Context("When a logpipeline with no input enabled exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())
			})

			k8sObjectsToCreate := append(k8sObjects, &logPipelineWithInput)
			Expect(kitk8s.CreateObjects(suite.Ctx, suite.K8sClient, k8sObjectsToCreate...)).Should(Succeed())
		})

		It("Ensures the log gateway deployment is ready", func() {
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, kitkyma.LogGatewayName)
		})

		It("Should have a logs backend running", func() {
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
			assert.ServiceReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Should have running pipelines", func() {
			assert.LogPipelineHealthy(suite.Ctx, suite.K8sClient, pipelineNameNoInput)
			assert.LogPipelineHealthy(suite.Ctx, suite.K8sClient, pipelineNameWithInput)
		})

		It("Pipeline with no input should have AgentNotRequired condition", func() {
			assert.LogPipelineHasCondition(suite.Ctx, suite.K8sClient, pipelineNameNoInput, metav1.Condition{
				Type:   conditions.TypeAgentHealthy,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonLogAgentNotRequired,
			})
		})

		It("Ensures the log agent DaemonSet is running", func() {
			assert.DaemonSetReady(suite.Ctx, suite.K8sClient, kitkyma.LogAgentName)
		})

		It("Should delete the pipeline with input", func() {
			Expect(kitk8s.DeleteObjects(suite.Ctx, suite.K8sClient, &logPipelineWithInput)).Should(Succeed())
		})

		It("Ensures the log agent DaemonSet is no longer running", func() {
			assert.DaemonSetNotFound(suite.Ctx, suite.K8sClient, kitkyma.LogAgentName)
		})
	})
})
