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
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestNamespaceSelector_OTel(t *testing.T) {
	tests := []struct {
		label               string
		inputBuilder        func(includeNs, excludeNs string) telemetryv1alpha1.LogPipelineInput
		logGeneratorBuilder func(ns string) client.Object
		expectAgent         bool
	}{
		{
			label: suite.LabelLogAgent,
			inputBuilder: func(includeNs, excludeNs string) telemetryv1alpha1.LogPipelineInput {
				var opts []testutils.ExtendedNamespaceSelectorOptions
				if includeNs != "" {
					opts = append(opts, testutils.ExtIncludeNamespaces(includeNs))
				}
				if excludeNs != "" {
					opts = append(opts, testutils.ExtExcludeNamespaces(excludeNs))
				}

				return testutils.BuildLogPipelineApplicationInput(opts...)
			},
			logGeneratorBuilder: func(ns string) client.Object {
				return loggen.New(ns).K8sObject()
			},
			expectAgent: true,
		},
		{
			label: suite.LabelLogGateway,
			inputBuilder: func(includeNs, excludeNs string) telemetryv1alpha1.LogPipelineInput {
				var opts []testutils.NamespaceSelectorOptions
				if includeNs != "" {
					opts = append(opts, testutils.IncludeNamespaces(includeNs))
				}
				if excludeNs != "" {
					opts = append(opts, testutils.ExcludeNamespaces(excludeNs))
				}

				return testutils.BuildLogPipelineOTLPInput(opts...)
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
				uniquePrefix            = unique.Prefix(tc.label)
				gen1Ns                  = uniquePrefix("gen-1")
				includeGen1PipelineName = uniquePrefix("include")
				gen2Ns                  = uniquePrefix("gen-2")
				excludeGen2PipelineName = uniquePrefix("exclude")
				backendNs               = uniquePrefix("backend")
			)

			backend1 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithName("backend-1"))
			backend1ExportURL := backend1.ExportURL(suite.ProxyClient)

			backend2 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithName("backend-2"))
			backend2ExportURL := backend2.ExportURL(suite.ProxyClient)

			includeGen1Pipeline := testutils.NewLogPipelineBuilder().
				WithName(includeGen1PipelineName).
				WithInput(tc.inputBuilder(gen1Ns, "")).
				WithOTLPOutput(testutils.OTLPEndpoint(backend1.Endpoint())).
				Build()

			excludeGen2Pipeline := testutils.NewLogPipelineBuilder().
				WithName(excludeGen2PipelineName).
				WithInput(tc.inputBuilder("", gen2Ns)).
				WithOTLPOutput(testutils.OTLPEndpoint(backend2.Endpoint())).
				Build()

			var resources []client.Object
			resources = append(resources,
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(gen1Ns).K8sObject(),
				kitk8s.NewNamespace(gen2Ns).K8sObject(),
				&includeGen1Pipeline,
				&excludeGen2Pipeline,
				tc.logGeneratorBuilder(gen1Ns),
				tc.logGeneratorBuilder(gen2Ns),
			)
			resources = append(resources, backend1.K8sObjects()...)
			resources = append(resources, backend2.K8sObjects()...)

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
			})
			Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

			assert.DeploymentReady(t.Context(), suite.K8sClient, kitkyma.LogGatewayName)
			assert.DeploymentReady(t.Context(), suite.K8sClient, backend1.NamespacedName())
			assert.DeploymentReady(t.Context(), suite.K8sClient, backend2.NamespacedName())

			if tc.expectAgent {
				assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.LogAgentName)
			}

			assert.OTelLogPipelineHealthy(t.Context(), suite.K8sClient, includeGen1PipelineName)
			assert.OTelLogPipelineHealthy(t.Context(), suite.K8sClient, excludeGen2PipelineName)

			assert.OTelLogsFromNamespaceDelivered(suite.ProxyClient, backend1ExportURL, gen1Ns)
			assert.OTelLogsFromNamespaceNotDelivered(suite.ProxyClient, backend2ExportURL, gen2Ns)
		})
	}
}

func TestNamespaceSelector_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix            = unique.Prefix()
		gen1Ns                  = uniquePrefix("gen-1")
		includeGen1PipelineName = uniquePrefix("include")
		gen2Ns                  = uniquePrefix("gen-2")
		excludeGen2PipelineName = uniquePrefix("exclude")
		backendNs               = uniquePrefix("backend")
	)

	backend1 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithName("backend-1"))
	backend1ExportURL := backend1.ExportURL(suite.ProxyClient)

	backend2 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithName("backend-2"))
	backend2ExportURL := backend2.ExportURL(suite.ProxyClient)

	includeGen1Pipeline := testutils.NewLogPipelineBuilder().
		WithName(includeGen1PipelineName).
		WithApplicationInput(true, testutils.ExtIncludeNamespaces(gen1Ns)).
		WithHTTPOutput(testutils.HTTPHost(backend1.Host()), testutils.HTTPPort(backend1.Port())).
		Build()

	excludeGen2Pipeline := testutils.NewLogPipelineBuilder().
		WithName(excludeGen2PipelineName).
		WithApplicationInput(true, testutils.ExtExcludeNamespaces(gen2Ns)).
		WithHTTPOutput(testutils.HTTPHost(backend2.Host()), testutils.HTTPPort(backend2.Port())).
		Build()

	var resources []client.Object
	resources = append(resources,
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(gen1Ns).K8sObject(),
		kitk8s.NewNamespace(gen2Ns).K8sObject(),
		&includeGen1Pipeline,
		&excludeGen2Pipeline,
		loggen.New(gen1Ns).K8sObject(),
		loggen.New(gen2Ns).K8sObject(),
	)
	resources = append(resources, backend1.K8sObjects()...)
	resources = append(resources, backend2.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, includeGen1PipelineName)
	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, excludeGen2PipelineName)

	assert.DeploymentReady(t.Context(), suite.K8sClient, backend1.NamespacedName())
	assert.DeploymentReady(t.Context(), suite.K8sClient, backend2.NamespacedName())

	assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.FluentBitDaemonSetName)

	assert.FluentBitLogsFromNamespaceDelivered(suite.ProxyClient, backend1ExportURL, gen1Ns)
	assert.FluentBitLogsFromNamespaceNotDelivered(suite.ProxyClient, backend2ExportURL, gen2Ns)
}
