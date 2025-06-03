package shared

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestNamespaceSelector_OTel(t *testing.T) {
	tests := []struct {
		label               string
		inputBuilder        func(includeNss, excludeNss []string) telemetryv1alpha1.LogPipelineInput
		logGeneratorBuilder func(ns string) client.Object
		expectAgent         bool
	}{
		{
			label: suite.LabelLogAgent,
			inputBuilder: func(includeNss, excludeNss []string) telemetryv1alpha1.LogPipelineInput {
				var opts []testutils.ExtendedNamespaceSelectorOptions
				if len(includeNss) > 0 {
					opts = append(opts, testutils.ExtIncludeNamespaces(includeNss...))
				}
				if len(excludeNss) > 0 {
					opts = append(opts, testutils.ExtExcludeNamespaces(excludeNss...))
				}

				return testutils.BuildLogPipelineApplicationInput(opts...)
			},
			logGeneratorBuilder: func(ns string) client.Object {
				return stdloggen.NewDeployment(ns).K8sObject()
			},
			expectAgent: true,
		},
		{
			label: suite.LabelLogGateway,
			inputBuilder: func(includeNss, excludeNss []string) telemetryv1alpha1.LogPipelineInput {
				var opts []testutils.NamespaceSelectorOptions
				if len(includeNss) > 0 {
					opts = append(opts, testutils.IncludeNamespaces(includeNss...))
				}
				if len(excludeNss) > 0 {
					opts = append(opts, testutils.ExcludeNamespaces(excludeNss...))
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
				uniquePrefix        = unique.Prefix(tc.label)
				gen1Ns              = uniquePrefix("gen-1")
				includePipelineName = uniquePrefix("include")
				gen2Ns              = uniquePrefix("gen-2")
				excludePipelineName = uniquePrefix("exclude")
				backendNs           = uniquePrefix("backend")
			)

			backend1 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithName("backend-1"))
			backend2 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithName("backend-2"))

			// Include gen1Ns only
			includePipeline := testutils.NewLogPipelineBuilder().
				WithName(includePipelineName).
				WithInput(tc.inputBuilder([]string{gen1Ns}, nil)).
				WithOTLPOutput(testutils.OTLPEndpoint(backend1.Endpoint())).
				Build()

			// Exclude all namespaces except gen1Ns (gen2Ns and other unrelated namespaces)
			// to avoid implicitly collecting logs from other namespaces
			// and potentially overloading the backend.
			var nsList corev1.NamespaceList

			Expect(suite.K8sClient.List(t.Context(), &nsList)).Should(Succeed())

			excludeNss := []string{gen2Ns}

			for _, namespace := range nsList.Items {
				if namespace.Name != gen1Ns && namespace.Name != gen2Ns {
					excludeNss = append(excludeNss, namespace.Name)
				}
			}

			excludePipeline := testutils.NewLogPipelineBuilder().
				WithName(excludePipelineName).
				WithInput(tc.inputBuilder(nil, excludeNss)).
				WithOTLPOutput(testutils.OTLPEndpoint(backend2.Endpoint())).
				Build()

			var resources []client.Object
			resources = append(resources,
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(gen1Ns).K8sObject(),
				kitk8s.NewNamespace(gen2Ns).K8sObject(),
				&includePipeline,
				&excludePipeline,
				tc.logGeneratorBuilder(gen1Ns),
				tc.logGeneratorBuilder(gen2Ns),
			)
			resources = append(resources, backend1.K8sObjects()...)
			resources = append(resources, backend2.K8sObjects()...)

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
			})
			Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

			assert.DeploymentReady(t.Context(), kitkyma.LogGatewayName)
			assert.DeploymentReady(t.Context(), backend1.NamespacedName())
			assert.DeploymentReady(t.Context(), backend2.NamespacedName())

			if tc.expectAgent {
				assert.DaemonSetReady(t.Context(), kitkyma.LogAgentName)
			}

			assert.OTelLogPipelineHealthy(t, includePipelineName)
			assert.OTelLogPipelineHealthy(t, excludePipelineName)

			assert.OTelLogsFromNamespaceDelivered(t, backend1, gen1Ns)
			assert.OTelLogsFromNamespaceNotDelivered(t, backend2, gen2Ns)
		})
	}
}

func TestNamespaceSelector_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix        = unique.Prefix()
		gen1Ns              = uniquePrefix("gen-1")
		includePipelineName = uniquePrefix("include")
		gen2Ns              = uniquePrefix("gen-2")
		excludePipelineName = uniquePrefix("exclude")
		backendNs           = uniquePrefix("backend")
	)

	backend1 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithName("backend-1"))
	backend2 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithName("backend-2"))

	includePipeline := testutils.NewLogPipelineBuilder().
		WithName(includePipelineName).
		WithApplicationInput(true, testutils.ExtIncludeNamespaces(gen1Ns)).
		WithHTTPOutput(testutils.HTTPHost(backend1.Host()), testutils.HTTPPort(backend1.Port())).
		Build()

	excludeGen2Pipeline := testutils.NewLogPipelineBuilder().
		WithName(excludePipelineName).
		WithApplicationInput(true, testutils.ExtExcludeNamespaces(gen2Ns)).
		WithHTTPOutput(testutils.HTTPHost(backend2.Host()), testutils.HTTPPort(backend2.Port())).
		Build()

	var resources []client.Object
	resources = append(resources,
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(gen1Ns).K8sObject(),
		kitk8s.NewNamespace(gen2Ns).K8sObject(),
		&includePipeline,
		&excludeGen2Pipeline,
		stdloggen.NewDeployment(gen1Ns).K8sObject(),
		stdloggen.NewDeployment(gen2Ns).K8sObject(),
	)
	resources = append(resources, backend1.K8sObjects()...)
	resources = append(resources, backend2.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

	assert.FluentBitLogPipelineHealthy(t, includePipelineName)
	assert.FluentBitLogPipelineHealthy(t, excludePipelineName)

	assert.DeploymentReady(t.Context(), backend1.NamespacedName())
	assert.DeploymentReady(t.Context(), backend2.NamespacedName())

	assert.DaemonSetReady(t.Context(), kitkyma.FluentBitDaemonSetName)

	assert.FluentBitLogsFromNamespaceDelivered(t, backend1, gen1Ns)
	assert.FluentBitLogsFromNamespaceNotDelivered(t, backend2, gen2Ns)
}
