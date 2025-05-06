package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
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
		name                 string
		logPipelineInputFunc func(includeNs, excludeNs string) telemetryv1alpha1.LogPipelineInput
		logGeneratorFunc     func(namespace string) client.Object
		agent                bool
	}{
		{
			name:                 "gateway",
			logPipelineInputFunc: withOTLPInput,
			logGeneratorFunc: func(namespace string) client.Object {
				return telemetrygen.NewDeployment(namespace, telemetrygen.SignalTypeLogs).K8sObject()
			},
		},

		{
			name:                 "agent",
			logPipelineInputFunc: withApplicationInput,
			logGeneratorFunc: func(namespace string) client.Object {
				return loggen.New(namespace).K8sObject()
			},
			agent: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				uniquePrefix        = unique.Prefix(tc.name)
				includeNs           = uniquePrefix("gen-include")
				includePipelineName = uniquePrefix("include")
				excludeNs           = uniquePrefix("gen-exclude")
				excludePipelineName = uniquePrefix("exclude")
				backendNs           = uniquePrefix("backend")
			)

			backendName := "backend"
			backend := backend.New(backendNs, backend.SignalTypeLogsOtel, backend.WithName(backendName))
			backendExportURL := backend.ExportURL(suite.ProxyClient)

			pipelineInclude := testutils.NewLogPipelineBuilder().
				WithName(includePipelineName).
				WithInput(tc.logPipelineInputFunc(includeNs, "")).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
				Build()

			pipelineExclude := testutils.NewLogPipelineBuilder().
				WithName(excludePipelineName).
				WithInput(tc.logPipelineInputFunc("", excludeNs)).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
				Build()

			var resources []client.Object
			resources = append(resources,
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(includeNs).K8sObject(),
				kitk8s.NewNamespace(excludeNs).K8sObject(),
				&pipelineInclude,
				&pipelineExclude,
				tc.logGeneratorFunc(includeNs),
				tc.logGeneratorFunc(excludeNs),
			)
			resources = append(resources, backend.K8sObjects()...)

			t.Cleanup(func() {
				err := kitk8s.DeleteObjects(t.Context(), suite.K8sClient, resources...)
				require.NoError(t, err)
			})
			Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

			assert.DeploymentReady(t.Context(), suite.K8sClient, kitkyma.LogGatewayName)
			assert.DeploymentReady(t.Context(), suite.K8sClient, types.NamespacedName{Name: backendName, Namespace: backendNs})

			if tc.agent {
				assert.DaemonSetReady(suite.Ctx, suite.K8sClient, kitkyma.LogAgentName)
			}

			assert.LogPipelineHealthy(t.Context(), suite.K8sClient, includePipelineName)
			assert.LogPipelineHealthy(t.Context(), suite.K8sClient, excludePipelineName)

			assert.OTelLogsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, includeNs)
			assert.OTelLogsFromNamespaceNotDelivered(suite.ProxyClient, backendExportURL, excludeNs)
		})
	}
}

func TestNamespaceSelector_FluentBit(t *testing.T) {
	RegisterTestingT(t)
	// suite.SkipIfDoesNotMatchLabel(t, "logs")

	var (
		uniquePrefix        = unique.Prefix()
		includeNs           = uniquePrefix("gen-include")
		includePipelineName = uniquePrefix("include")
		excludeNs           = uniquePrefix("gen-exclude")
		excludePipelineName = uniquePrefix("exclude")
		backendNs           = uniquePrefix("backend")
	)

	backendName := "backend"
	backend := backend.New(backendNs, backend.SignalTypeLogsFluentBit, backend.WithName(backendName))
	backendExportURL := backend.ExportURL(suite.ProxyClient)

	pipelineInclude := testutils.NewLogPipelineBuilder().
		WithName(includePipelineName).
		WithInput(withApplicationInput(includeNs, "")).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
		Build()

	pipelineExclude := testutils.NewLogPipelineBuilder().
		WithName(excludePipelineName).
		WithInput(withApplicationInput("", excludeNs)).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
		Build()

	var resources []client.Object
	resources = append(resources,
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(includeNs).K8sObject(),
		kitk8s.NewNamespace(excludeNs).K8sObject(),
		&pipelineInclude,
		&pipelineExclude,
		loggen.New(includeNs).K8sObject(),
		loggen.New(excludeNs).K8sObject(),
	)
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		err := kitk8s.DeleteObjects(t.Context(), suite.K8sClient, resources...)
		require.NoError(t, err)
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	assert.LogPipelineHealthy(t.Context(), suite.K8sClient, includePipelineName)
	assert.LogPipelineHealthy(t.Context(), suite.K8sClient, excludePipelineName)
	assert.DeploymentReady(t.Context(), suite.K8sClient, types.NamespacedName{Name: backendName, Namespace: backendNs})
	assert.DaemonSetReady(suite.Ctx, suite.K8sClient, kitkyma.FluentBitDaemonSetName)

	assert.FluentBitLogsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, includeNs)
	assert.FluentBitLogsFromNamespaceNotDelivered(suite.ProxyClient, backendExportURL, excludeNs)
}

func withApplicationInput(includeNs, excludeNs string) telemetryv1alpha1.LogPipelineInput {
	if includeNs != "" {
		return telemetryv1alpha1.LogPipelineInput{
			Application: &telemetryv1alpha1.LogPipelineApplicationInput{
				Enabled: ptr.To(true),
				Namespaces: telemetryv1alpha1.LogPipelineNamespaceSelector{
					Include: []string{includeNs},
				},
			},
		}
	}

	return telemetryv1alpha1.LogPipelineInput{
		Application: &telemetryv1alpha1.LogPipelineApplicationInput{
			Enabled: ptr.To(true),
			Namespaces: telemetryv1alpha1.LogPipelineNamespaceSelector{
				Exclude: []string{excludeNs},
			},
		},
	}
}

func withOTLPInput(includeNs, excludeNs string) telemetryv1alpha1.LogPipelineInput {
	if includeNs != "" {
		return telemetryv1alpha1.LogPipelineInput{
			Application: &telemetryv1alpha1.LogPipelineApplicationInput{
				Enabled: ptr.To(false),
			},
			OTLP: &telemetryv1alpha1.OTLPInput{
				Namespaces: &telemetryv1alpha1.NamespaceSelector{
					Include: []string{includeNs},
				},
			},
		}
	}

	return telemetryv1alpha1.LogPipelineInput{
		Application: &telemetryv1alpha1.LogPipelineApplicationInput{
			Enabled: ptr.To(false),
		},
		OTLP: &telemetryv1alpha1.OTLPInput{
			Namespaces: &telemetryv1alpha1.NamespaceSelector{
				Exclude: []string{excludeNs},
			},
		},
	}
}
