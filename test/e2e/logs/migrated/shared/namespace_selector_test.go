package shared

import (
	"context"
	"fmt"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"testing"
)

func TestPipelineNamespace_Otel(t *testing.T) {
	RegisterTestingT(t)
	//suite.SkipIfDoesNotMatchLabel(t, "logs")

	tests := []struct {
		name string

		logPipelineInputFunc func(includeNs, excludeNs string) telemetryv1alpha1.LogPipelineInput
		logProducerFunc      func(deploymentName, namespace string) client.Object

		agent bool
	}{
		{
			name: "gateway",
			logPipelineInputFunc: func(includeNs, excludeNs string) telemetryv1alpha1.LogPipelineInput {
				return withOTLPInput(includeNs, excludeNs)
			},

			logProducerFunc: func(deploymentName, namespace string) client.Object {
				podSpecWithUndefinedService := telemetrygen.PodSpec(telemetrygen.SignalTypeLogs, telemetrygen.WithServiceName(""))
				return kitk8s.NewDeployment(deploymentName, namespace).
					WithLabel("app", "workload").
					WithPodSpec(podSpecWithUndefinedService).
					K8sObject()
			},
		},

		{
			name: "agent",
			logPipelineInputFunc: func(includeNs, excludeNs string) telemetryv1alpha1.LogPipelineInput {
				return withApplicationInput(includeNs, excludeNs)
			},

			logProducerFunc: func(deploymentName, namespace string) client.Object {
				return loggen.New(namespace).
					K8sObject()
			},
			agent: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				includeNs = "producer-include"
				excludeNs = "producer-exclude"
				backendNs = suite.IDWithSuffix("backend")
			)

			if tc.agent {
				includeNs = fmt.Sprintf("%s-agent", includeNs)
				excludeNs = fmt.Sprintf("%s-agent", excludeNs)
				backendNs = fmt.Sprintf("%s-agent", backendNs)
			}

			backendName := "backend"
			backendObj := backend.New(backendNs, backend.SignalTypeLogsOtel, backend.WithName(backendName))
			backendExportURL := backendObj.ExportURL(suite.ProxyClient)

			logPipelineOutput := telemetryv1alpha1.LogPipelineOutput{
				OTLP: &telemetryv1alpha1.OTLPOutput{
					Endpoint: telemetryv1alpha1.ValueType{Value: backendObj.Endpoint()},
				},
			}

			pipelineIncludeName := fmt.Sprintf("%s-include", tc.name)
			pipelineInclude := testutils.NewLogPipelineBuilder().
				WithName(pipelineIncludeName).
				WithInput(tc.logPipelineInputFunc(includeNs, "")).
				WithOutput(logPipelineOutput).
				Build()

			pipelineExcludeName := fmt.Sprintf("%s-exclude", tc.name)
			pipelineExclude := testutils.NewLogPipelineBuilder().
				WithName(pipelineExcludeName).
				WithInput(tc.logPipelineInputFunc("", excludeNs)).
				WithOutput(logPipelineOutput).
				Build()

			var resources []client.Object
			resources = append(resources,
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(includeNs).K8sObject(),
				kitk8s.NewNamespace(excludeNs).K8sObject(),
				&pipelineInclude,
				&pipelineExclude,
				tc.logProducerFunc("foo", includeNs),
				tc.logProducerFunc("bar", excludeNs),
			)
			resources = append(resources, backendObj.K8sObjects()...)

			t.Cleanup(func() {
				err := kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)
				require.NoError(t, err)
			})
			Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

			t.Log("Waiting for resources to be ready")

			assert.LogPipelineHealthy(t.Context(), suite.K8sClient, pipelineIncludeName)
			assert.LogPipelineHealthy(t.Context(), suite.K8sClient, pipelineExcludeName)
			assert.DeploymentReady(t.Context(), suite.K8sClient, kitkyma.LogGatewayName)
			assert.DeploymentReady(t.Context(), suite.K8sClient, types.NamespacedName{Name: backendName, Namespace: backendNs})

			if tc.agent {
				assert.DaemonSetReady(suite.Ctx, suite.K8sClient, kitkyma.LogAgentName)
			}

			assert.OtelLogsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, includeNs)
			assert.OtelLogsFromNamespaceNotDelivered(suite.ProxyClient, backendExportURL, excludeNs)

		})

	}
}

func TestPipelineNamespace_FluentBit(t *testing.T) {
	RegisterTestingT(t)
	//suite.SkipIfDoesNotMatchLabel(t, "logs")

	var (
		name        = "namespace-selector-fluent-bit"
		includeNs   = "producer-include-fluentbit"
		excludeNs   = "producer-exclude-fluentbit"
		backendNs   = suite.IDWithSuffix("backend")
		backendName = "backend"
	)

	backendObj := backend.New(backendNs, backend.SignalTypeLogsFluentBit, backend.WithName(backendName))
	backendExportURL := backendObj.ExportURL(suite.ProxyClient)

	logPipelineOutputFunc := telemetryv1alpha1.LogPipelineOutput{
		HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{
			Host: telemetryv1alpha1.ValueType{Value: backendObj.Host()},
			Port: strconv.Itoa(int(backendObj.Port())),
		},
	}

	pipelineIncludeName := fmt.Sprintf("%s-include", name)
	pipelineInclude := testutils.NewLogPipelineBuilder().
		WithName(pipelineIncludeName).
		WithInput(withApplicationInput(includeNs, "")).
		WithOutput(logPipelineOutputFunc).
		Build()

	pipelineExcludeName := fmt.Sprintf("%s-exclude", name)
	pipelineExclude := testutils.NewLogPipelineBuilder().
		WithName(pipelineExcludeName).
		WithInput(withApplicationInput("", excludeNs)).
		WithHTTPOutput(testutils.HTTPHost(backendObj.Host()), testutils.HTTPPort(backendObj.Port())).
		Build()

	var resources []client.Object
	resources = append(resources,
		kitk8s.NewNamespace(backendNs).K8sObject(),
		&pipelineInclude,
		&pipelineExclude,
		kitk8s.NewNamespace(includeNs).K8sObject(),
		kitk8s.NewNamespace(excludeNs).K8sObject(),
		loggen.New(includeNs).K8sObject(),
		loggen.New(excludeNs).K8sObject(),
	)
	resources = append(resources, backendObj.K8sObjects()...)

	t.Cleanup(func() {
		err := kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)
		require.NoError(t, err)
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	t.Log("Waiting for resources to be ready")

	assert.LogPipelineHealthy(t.Context(), suite.K8sClient, pipelineIncludeName)
	assert.LogPipelineHealthy(t.Context(), suite.K8sClient, pipelineExcludeName)
	assert.DeploymentReady(t.Context(), suite.K8sClient, types.NamespacedName{Name: backendName, Namespace: backendNs})
	assert.DaemonSetReady(suite.Ctx, suite.K8sClient, kitkyma.FluentBitDaemonSetName)

	assert.FBLogsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, includeNs)
	assert.FBLogsFromNamespaceNotDelivered(suite.ProxyClient, backendExportURL, excludeNs)

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
