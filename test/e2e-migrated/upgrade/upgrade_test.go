package upgrade

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

// LogPipeline upgrade test flow
// Metric and TracePipeline tests are still written in the old style and will be
// migrated to the new style in the future.
func TestUpgrade(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelUpgrade)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		generatorNs  = uniquePrefix("gen")
		backendNs    = uniquePrefix("backend")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithPersistentHostSecret(true))
	backendExportURL := backend.ExportURL(suite.ProxyClient)

	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(generatorNs).K8sObject(),
		loggen.New(generatorNs).K8sObject(),
		&pipeline,
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Run("before upgrade", func(t *testing.T) {
		require.NoError(t, kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...))

		assert.DeploymentReady(t.Context(), suite.K8sClient, backend.NamespacedName())
		assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.FluentBitDaemonSetName)

		assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineName)
		assert.FluentBitLogsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, generatorNs)
	})

	t.Run("after upgrade", func(t *testing.T) {
		t.Cleanup(func() {
			require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
		})

		assert.DeploymentReady(t.Context(), suite.K8sClient, backend.NamespacedName())
		assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.FluentBitDaemonSetName)

		assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineName)
		assert.FluentBitLogsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, generatorNs)
	})
}
