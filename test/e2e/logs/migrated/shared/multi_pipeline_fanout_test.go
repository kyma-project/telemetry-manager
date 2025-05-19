package shared

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMultiPipelineFanout_OTel(t *testing.T) {
	tests := []struct {
		label               string
		inputBuilder        func(includeNs string) telemetryv1alpha1.LogPipelineInput
		logGeneratorBuilder func(ns string) client.Object
		expectAgent         bool
	}{
		{
			label: suite.LabelLogAgent,
			inputBuilder: func(includeNs string) telemetryv1alpha1.LogPipelineInput {
				return testutils.BuildLogPipelineApplicationInput(testutils.ExtIncludeNamespaces(includeNs))
			},
			logGeneratorBuilder: func(ns string) client.Object {
				return loggen.New(ns).K8sObject()
			},
			expectAgent: true,
		},
		{
			label: suite.LabelLogGateway,
			inputBuilder: func(includeNs string) telemetryv1alpha1.LogPipelineInput {
				return testutils.BuildLogPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
			logGeneratorBuilder: func(ns string) client.Object {
				return telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeLogs).K8sObject()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label)

			var (
				uniquePrefix  = unique.Prefix(tc.label)
				backendNs     = uniquePrefix("backend")
				genNs         = uniquePrefix("gen")
				pipeline1Name = uniquePrefix("pipeline1")
				pipeline2Name = uniquePrefix("pipeline2")
			)

			backend1 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithName("backend1"))
			backend2 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithName("backend2"))

			backend1ExportURL := backend1.ExportURL(suite.ProxyClient)
			backend2ExportURL := backend2.ExportURL(suite.ProxyClient)

			pipeline1 := testutils.NewLogPipelineBuilder().
				WithName(pipeline1Name).
				WithInput(tc.inputBuilder(genNs)).
				WithOTLPOutput(testutils.OTLPEndpoint(backend1.Endpoint())).
				Build()

			pipeline2 := testutils.NewLogPipelineBuilder().
				WithName(pipeline2Name).
				WithInput(tc.inputBuilder(genNs)).
				WithOTLPOutput(testutils.OTLPEndpoint(backend2.Endpoint())).
				Build()

			resources := []client.Object{
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(genNs).K8sObject(),
				&pipeline1,
				&pipeline2,
				tc.logGeneratorBuilder(genNs),
			}
			resources = append(resources, backend1.K8sObjects()...)
			resources = append(resources, backend2.K8sObjects()...)

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
			})
			require.NoError(t, kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...))

			assert.DeploymentReady(t.Context(), suite.K8sClient, backend1.NamespacedName())
			assert.DeploymentReady(t.Context(), suite.K8sClient, backend2.NamespacedName())

			assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipeline1.Name)
			assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipeline2.Name)

			assert.OTelLogsFromNamespaceDelivered(suite.ProxyClient, backend1ExportURL, genNs)
			assert.OTelLogsFromNamespaceDelivered(suite.ProxyClient, backend2ExportURL, genNs)
		})
	}
}

func TestMultiPipelineFanout_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix  = unique.Prefix()
		backendNs     = uniquePrefix("backend")
		genNs         = uniquePrefix("gen")
		pipeline1Name = uniquePrefix("pipeline1")
		pipeline2Name = uniquePrefix("pipeline2")
	)

	backend1 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithName("backend1"))
	backend2 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithName("backend2"))

	backend1ExportURL := backend1.ExportURL(suite.ProxyClient)
	backend2ExportURL := backend2.ExportURL(suite.ProxyClient)

	pipeline1 := testutils.NewLogPipelineBuilder().
		WithName(pipeline1Name).
		WithApplicationInput(true).
		WithHTTPOutput(testutils.HTTPHost(backend1.Host()), testutils.HTTPPort(backend1.Port())).
		Build()

	pipeline2 := testutils.NewLogPipelineBuilder().
		WithName(pipeline2Name).
		WithApplicationInput(true).
		WithHTTPOutput(testutils.HTTPHost(backend2.Host()), testutils.HTTPPort(backend2.Port())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&pipeline1,
		&pipeline2,
		loggen.New(genNs).K8sObject(),
	}
	resources = append(resources, backend1.K8sObjects()...)
	resources = append(resources, backend2.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	require.NoError(t, kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...))

	assert.DeploymentReady(t.Context(), suite.K8sClient, backend1.NamespacedName())
	assert.DeploymentReady(t.Context(), suite.K8sClient, backend2.NamespacedName())

	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipeline1.Name)
	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipeline2.Name)

	assert.FluentBitLogsFromNamespaceDelivered(suite.ProxyClient, backend1ExportURL, genNs)
	assert.FluentBitLogsFromNamespaceDelivered(suite.ProxyClient, backend2ExportURL, genNs)
}
