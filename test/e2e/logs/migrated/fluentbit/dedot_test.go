package fluentbit

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestDedot(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelLogsFluentBit)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		mockNs       = uniquePrefix()
	)

	backend := kitbackend.New(mockNs, kitbackend.SignalTypeLogsFluentBit)
	backendExportURL := backend.ExportURL(suite.ProxyClient)
	logProducer := loggen.New(mockNs).WithLabels(map[string]string{"dedot.label": "logging-dedot-value"})
	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithIncludeContainers(loggen.DefaultContainerName).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port()), testutils.HTTPDedot(true)).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(mockNs).K8sObject(),
		logProducer.K8sObject(),
		&pipeline,
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineName)
	assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.FluentBitDaemonSetName)
	assert.DeploymentReady(t.Context(), suite.K8sClient, types.NamespacedName{Namespace: mockNs, Name: kitbackend.DefaultName})
	assert.DeploymentReady(t.Context(), suite.K8sClient, types.NamespacedName{Namespace: mockNs, Name: loggen.DefaultName})

	assert.TelemetryDataDelivered(suite.ProxyClient, backendExportURL, HaveFlatFluentBitLogs(
		ContainElement(HaveKubernetesLabels(HaveKeyWithValue("dedot_label", "logging-dedot-value")))),
	)
}
