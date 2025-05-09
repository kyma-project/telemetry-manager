package fluentbit

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
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

func TestCustomOutput(t *testing.T) {
	RegisterTestingT(t)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		mockNs       = uniquePrefix()
	)

	backend := kitbackend.New(mockNs, kitbackend.SignalTypeLogsFluentBit)
	backendExportURL := backend.ExportURL(suite.ProxyClient)

	logProducer := loggen.New(mockNs)

	customOutputTemplate := fmt.Sprintf(`
	name   http
	port   %d
	host   %s
	format json`, backend.Port(), backend.Host())
	logPipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithCustomOutput(customOutputTemplate).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(mockNs).K8sObject(),
		logProducer.K8sObject(),
		&logPipeline,
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineName)
	assert.LogPipelineUnsupportedMode(t.Context(), suite.K8sClient, pipelineName, true)
	assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.FluentBitDaemonSetName)
	assert.DeploymentReady(t.Context(), suite.K8sClient, backend.NamespacedName())
	assert.FluentBitLogsFromPodDelivered(suite.ProxyClient, backendExportURL, loggen.DefaultName)
}
