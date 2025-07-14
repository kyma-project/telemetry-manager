package shared

import (
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdloggen"
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
				return stdloggen.NewDeployment(ns).K8sObject()
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
				require.NoError(t, kitk8s.DeleteObjects(resources...))
			})
			require.NoError(t, kitk8s.CreateObjects(t, resources...))

			assert.DeploymentReady(t, backend1.NamespacedName())
			assert.DeploymentReady(t, backend2.NamespacedName())

			assert.FluentBitLogPipelineHealthy(t, pipeline1.Name)
			assert.FluentBitLogPipelineHealthy(t, pipeline2.Name)

			assert.OTelLogsFromNamespaceDelivered(t, backend1, genNs)
			assert.OTelLogsFromNamespaceDelivered(t, backend2, genNs)
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
		stdloggen.NewDeployment(genNs).K8sObject(),
	}
	resources = append(resources, backend1.K8sObjects()...)
	resources = append(resources, backend2.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(resources...))
	})
	require.NoError(t, kitk8s.CreateObjects(t, resources...))

	assert.DeploymentReady(t, backend1.NamespacedName())
	assert.DeploymentReady(t, backend2.NamespacedName())

	assert.FluentBitLogPipelineHealthy(t, pipeline1.Name)
	assert.FluentBitLogPipelineHealthy(t, pipeline2.Name)

	assert.FluentBitLogsFromNamespaceDelivered(t, backend1, genNs)
	assert.FluentBitLogsFromNamespaceDelivered(t, backend2, genNs)
}
