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
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestNamespaceSelector_OTel(t *testing.T) {
	RegisterTestingT(t)
	// suite.SkipIfDoesNotMatchLabel(t, "logs")

	tests := []struct {
		name                string
		inputBuilder        func(includeNs, excludeNs string) telemetryv1alpha1.LogPipelineInput
		logGeneratorBuilder func(namespace string) client.Object
		expectAgent         bool
	}{
		{
			name: "agent",
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
			logGeneratorBuilder: func(namespace string) client.Object {
				return loggen.New(namespace).K8sObject()
			},
			expectAgent: true,
		},
		{
			name: "gateway",
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
			logGeneratorBuilder: func(namespace string) client.Object {
				return telemetrygen.NewDeployment(namespace, telemetrygen.SignalTypeLogs).K8sObject()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				uniquePrefix            = unique.Prefix(tc.name)
				gen1Ns                  = uniquePrefix("gen-1")
				includeGen1PipelineName = uniquePrefix("include")
				gen2Ns                  = uniquePrefix("gen-2")
				excludeGen2PipelineName = uniquePrefix("exclude")
				backendNs               = uniquePrefix("backend")
			)

			backend := backend.New(backendNs, backend.SignalTypeLogsOTel)
			backendExportURL := backend.ExportURL(suite.ProxyClient)

			includeGen1Pipeline := testutils.NewLogPipelineBuilder().
				WithName(includeGen1PipelineName).
				WithInput(tc.inputBuilder(gen1Ns, "")).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
				Build()

			excludeGen2Pipeline := testutils.NewLogPipelineBuilder().
				WithName(excludeGen2PipelineName).
				WithInput(tc.inputBuilder("", gen2Ns)).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
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
			resources = append(resources, backend.K8sObjects()...)

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
			})
			Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

			assert.DeploymentReady(t.Context(), suite.K8sClient, kitkyma.LogGatewayName)
			assert.DeploymentReady(t.Context(), suite.K8sClient, backend.NamespacedName())

			if tc.expectAgent {
				assert.DaemonSetReady(suite.Ctx, suite.K8sClient, kitkyma.LogAgentName)
			}

			assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, includeGen1PipelineName)
			assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, excludeGen2PipelineName)

			assert.OTelLogsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, gen1Ns)
			assert.OTelLogsFromNamespaceNotDelivered(suite.ProxyClient, backendExportURL, gen2Ns)
		})
	}
}

func TestNamespaceSelector_FluentBit(t *testing.T) {
	RegisterTestingT(t)
	// suite.SkipIfDoesNotMatchLabel(t, "logs")

	var (
		uniquePrefix            = unique.Prefix()
		gen1Ns                  = uniquePrefix("gen-1")
		includeGen1PipelineName = uniquePrefix("include")
		gen2Ns                  = uniquePrefix("gen-2")
		excludeGen2PipelineName = uniquePrefix("exclude")
		backendNs               = uniquePrefix("backend")
	)

	backend := backend.New(backendNs, backend.SignalTypeLogsFluentBit)
	backendExportURL := backend.ExportURL(suite.ProxyClient)

	includeGen1Pipeline := testutils.NewLogPipelineBuilder().
		WithName(includeGen1PipelineName).
		WithApplicationInput(true, testutils.ExtIncludeNamespaces(gen1Ns)).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
		Build()

	excludeGen2Pipeline := testutils.NewLogPipelineBuilder().
		WithName(excludeGen2PipelineName).
		WithApplicationInput(true, testutils.ExtExcludeNamespaces(gen2Ns)).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
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
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, includeGen1PipelineName)
	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, excludeGen2PipelineName)
	assert.DeploymentReady(t.Context(), suite.K8sClient, backend.NamespacedName())
	assert.DaemonSetReady(suite.Ctx, suite.K8sClient, kitkyma.FluentBitDaemonSetName)

	assert.FluentBitLogsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, gen1Ns)
	assert.FluentBitLogsFromNamespaceNotDelivered(suite.ProxyClient, backendExportURL, gen2Ns)
}
