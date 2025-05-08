package fluentbit

import (
	"context"
	"fmt"
	"testing"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestCustomOutput(t *testing.T) {
	RegisterTestingT(t)

	var (
		resources    []client.Object
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		mockNs       = uniquePrefix()
	)

	resources = append(resources, kitk8s.NewNamespace(mockNs).K8sObject())

	be := backend.New(mockNs, backend.SignalTypeLogsFluentBit)
	backendExportURL := be.ExportURL(suite.ProxyClient)
	resources = append(resources, be.K8sObjects()...)

	mockLogProducer := loggen.New(mockNs)
	resources = append(resources, mockLogProducer.K8sObject())

	customOutputTemplate := fmt.Sprintf(`
	name   http
	port   %d
	host   %s
	format json`, be.Port(), be.Host())
	logPipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithCustomOutput(customOutputTemplate).
		Build()
	resources = append(resources, &logPipeline)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineName)
	assert.LogPipelineUnsupportedMode(t.Context(), suite.K8sClient, pipelineName, true)
	assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.FluentBitDaemonSetName)
	assert.DeploymentReady(t.Context(), suite.K8sClient, be.NamespacedName())
	assert.DeploymentReady(t.Context(), suite.K8sClient, be.NamespacedName())
	assert.FluentBitLogsFromPodDelivered(suite.ProxyClient, backendExportURL, loggen.DefaultName)
}

func TestCustomFilter(t *testing.T) {
	RegisterTestingT(t)

	t.Run("shoud reject a logpipeline with denied custom filter", func(t *testing.T) {
		logPipeline := testutils.NewLogPipelineBuilder().
			WithName("denied-custom-filter-pipeline").
			WithCustomFilter("Name kubernetes").
			WithCustomOutput("Name stdout").
			Build()
		Consistently(func(g Gomega) {
			g.Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, &logPipeline)).ShouldNot(Succeed())
		}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(Succeed())
	})
}

// TODO: Positive test for custom filter
