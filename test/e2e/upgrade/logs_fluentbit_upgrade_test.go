package upgrade

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

// LogPipeline (FluentBit) upgrade test flow
func TestLogsFluentBitUpgrade(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelUpgrade)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit)

	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&pipeline,
		stdoutloggen.NewDeployment(genNs).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Run("before upgrade", func(t *testing.T) {
		Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

		assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
		assert.FluentBitLogPipelineHealthy(t, pipelineName)
		assert.BackendReachable(t, backend)
		assert.FluentBitLogsFromNamespaceDelivered(t, backend, genNs)
	})

	t.Run("after upgrade", func(t *testing.T) {
		// TODO(TeodorSAP): Delete after 1.47 release ---
		assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
		assert.FluentBitLogPipelineHealthy(t, "logs-upgrade")

		backend = kitbackend.New("logs-upgrade-backend", kitbackend.SignalTypeLogsFluentBit)
		assert.BackendReachable(t, backend)
		assert.FluentBitLogsFromNamespaceDelivered(t, backend, "logs-upgrade-gen")
		// ---

		// TODO(TeodorSAP): Uncomment after 1.47 release
		// assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
		// assert.FluentBitLogPipelineHealthy(t, pipelineName)
		// assert.BackendReachable(t, backend)
		// assert.FluentBitLogsFromNamespaceDelivered(t, backend, genNs)
	})
}
