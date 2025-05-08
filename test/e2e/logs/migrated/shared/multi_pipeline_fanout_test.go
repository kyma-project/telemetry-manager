package shared

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMultiPipelineFanout_OTel(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		name                string
		input               telemetryv1alpha1.LogPipelineInput
		logGeneratorBuilder func(namespace string) client.Object
		expectAgent         bool
	}{
		{
			name:  "agent",
			input: testutils.BuildLogPipelineApplicationInput(),
			logGeneratorBuilder: func(namespace string) client.Object {
				return loggen.New(namespace).K8sObject()
			},
			expectAgent: true,
		},
		{
			name:  "gateway",
			input: testutils.BuildLogPipelineOTLPInput(),
			logGeneratorBuilder: func(namespace string) client.Object {
				return telemetrygen.NewDeployment(namespace, telemetrygen.SignalTypeLogs).K8sObject()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				uniquePrefix  = unique.Prefix(tc.name)
				backendNs     = uniquePrefix("backend")
				generatorNs   = uniquePrefix("gen")
				pipeline1Name = uniquePrefix("pipeline1")
				pipeline2Name = uniquePrefix("pipeline2")
			)

			backend1 := backend.New(backendNs, backend.SignalTypeLogsOTel, backend.WithName("backend1"))
			backend2 := backend.New(backendNs, backend.SignalTypeLogsOTel, backend.WithName("backend2"))

			backend1ExportURL := backend1.ExportURL(suite.ProxyClient)
			backend2ExportURL := backend2.ExportURL(suite.ProxyClient)

			logPipeline1 := testutils.NewLogPipelineBuilder().
				WithName(pipeline1Name).
				WithInput(tc.input).
				WithOTLPOutput(testutils.OTLPEndpoint(backend1.Endpoint())).
				Build()

			logPipeline2 := testutils.NewLogPipelineBuilder().
				WithName(pipeline2Name).
				WithInput(tc.input).
				WithOTLPOutput(testutils.OTLPEndpoint(backend2.Endpoint())).
				Build()

			resources := []client.Object{
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(generatorNs).K8sObject(),
				&logPipeline1,
				&logPipeline2,
				tc.logGeneratorBuilder(generatorNs),
			}
			resources = append(resources, backend1.K8sObjects()...)
			resources = append(resources, backend2.K8sObjects()...)

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
			})
			require.NoError(t, kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...))

			assert.DeploymentReady(t.Context(), suite.K8sClient, backend1.NamespacedName())
			assert.DeploymentReady(t.Context(), suite.K8sClient, backend2.NamespacedName())

			assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, logPipeline1.Name)
			assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, logPipeline2.Name)

			assert.OTelLogsFromNamespaceDelivered(suite.ProxyClient, backend1ExportURL, generatorNs)
			assert.OTelLogsFromNamespaceDelivered(suite.ProxyClient, backend2ExportURL, generatorNs)
		})
	}
}

func TestMultiPipelineFanout_FluentBit(t *testing.T) {
	RegisterTestingT(t)

	var (
		uniquePrefix  = unique.Prefix()
		backendNs     = uniquePrefix("backend")
		generatorNs   = uniquePrefix("gen")
		pipeline1Name = uniquePrefix("pipeline1")
		pipeline2Name = uniquePrefix("pipeline2")
	)

	backend1 := backend.New(backendNs, backend.SignalTypeLogsFluentBit, backend.WithName("backend1"))
	backend2 := backend.New(backendNs, backend.SignalTypeLogsFluentBit, backend.WithName("backend2"))

	backend1ExportURL := backend1.ExportURL(suite.ProxyClient)
	backend2ExportURL := backend2.ExportURL(suite.ProxyClient)

	logPipeline1 := testutils.NewLogPipelineBuilder().
		WithName(pipeline1Name).
		WithApplicationInput(true).
		WithHTTPOutput(testutils.HTTPHost(backend1.Host()), testutils.HTTPPort(backend1.Port())).
		Build()

	logPipeline2 := testutils.NewLogPipelineBuilder().
		WithName(pipeline2Name).
		WithApplicationInput(true).
		WithHTTPOutput(testutils.HTTPHost(backend2.Host()), testutils.HTTPPort(backend2.Port())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(generatorNs).K8sObject(),
		&logPipeline1,
		&logPipeline2,
		loggen.New(generatorNs).K8sObject(),
	}
	resources = append(resources, backend1.K8sObjects()...)
	resources = append(resources, backend2.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	require.NoError(t, kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...))

	assert.DeploymentReady(t.Context(), suite.K8sClient, backend1.NamespacedName())
	assert.DeploymentReady(t.Context(), suite.K8sClient, backend2.NamespacedName())

	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, logPipeline1.Name)
	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, logPipeline2.Name)

	assert.FluentBitLogsFromNamespaceDelivered(suite.ProxyClient, backend1ExportURL, generatorNs)
	assert.FluentBitLogsFromNamespaceDelivered(suite.ProxyClient, backend2ExportURL, generatorNs)
}
