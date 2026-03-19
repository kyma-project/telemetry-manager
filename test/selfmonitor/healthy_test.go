package selfmonitor

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	"github.com/kyma-project/telemetry-manager/test/testkit"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestHealthy(t *testing.T) {
	tests := []struct {
		name      string
		component string
		generator func(ns string) []client.Object
		assert    func(t *testing.T, ns string, backend *kitbackend.Backend, pipelineName string)
	}{
		{
			name:      "log-agent",
			component: suite.LabelLogAgent,
			generator: stdoutLogGeneratorDefault(),
			assert: func(t *testing.T, ns string, backend *kitbackend.Backend, pipelineName string) {
				assertComponentReady(t, suite.LabelLogAgent)
				assertPipelineHealthy(t, suite.LabelLogAgent, pipelineName)
				assert.OTelLogsFromNamespaceDelivered(t, backend, ns)
				assert.LogPipelineSelfMonitorIsHealthy(t, suite.K8sClient, pipelineName)
			},
		},
		{
			name:      "log-gateway",
			component: suite.LabelLogGateway,
			generator: otelGenerator(telemetrygen.SignalTypeLogs),
			assert: func(t *testing.T, ns string, backend *kitbackend.Backend, pipelineName string) {
				assertComponentReady(t, suite.LabelLogGateway)
				assertPipelineHealthy(t, suite.LabelLogGateway, pipelineName)
				assert.OTelLogsFromNamespaceDelivered(t, backend, ns)
				assert.LogPipelineSelfMonitorIsHealthy(t, suite.K8sClient, pipelineName)
			},
		},
		{
			name:      "fluent-bit",
			component: suite.LabelFluentBit,
			generator: stdoutLogGeneratorDefault(),
			assert: func(t *testing.T, ns string, backend *kitbackend.Backend, pipelineName string) {
				assertComponentReady(t, suite.LabelFluentBit)
				assertPipelineHealthy(t, suite.LabelFluentBit, pipelineName)
				assert.FluentBitLogsFromNamespaceDelivered(t, backend, ns)
				assert.LogPipelineSelfMonitorIsHealthy(t, suite.K8sClient, pipelineName)
			},
		},
		{
			name:      "metric-gateway",
			component: suite.LabelMetricGateway,
			generator: otelGenerator(telemetrygen.SignalTypeMetrics),
			assert: func(t *testing.T, ns string, backend *kitbackend.Backend, pipelineName string) {
				assertComponentReady(t, suite.LabelMetricGateway)
				assertPipelineHealthy(t, suite.LabelMetricGateway, pipelineName)
				assert.MetricsFromNamespaceDelivered(t, backend, ns, telemetrygen.MetricNames)
				assert.MetricPipelineSelfMonitorIsHealthy(t, suite.K8sClient, pipelineName)
			},
		},
		{
			name:      "metric-agent",
			component: suite.LabelMetricAgent,
			generator: promMetricGenerator(),
			assert: func(t *testing.T, ns string, backend *kitbackend.Backend, pipelineName string) {
				assertComponentReady(t, suite.LabelMetricAgent)
				assertPipelineHealthy(t, suite.LabelMetricAgent, pipelineName)
				assert.MetricsFromNamespaceDelivered(t, backend, ns, prommetricgen.CustomMetricNames())
				assert.MetricPipelineSelfMonitorIsHealthy(t, suite.K8sClient, pipelineName)
			},
		},
		{
			name:      "traces",
			component: suite.LabelTraces,
			generator: otelGenerator(telemetrygen.SignalTypeTraces),
			assert: func(t *testing.T, ns string, backend *kitbackend.Backend, pipelineName string) {
				assertComponentReady(t, suite.LabelTraces)
				assertPipelineHealthy(t, suite.LabelTraces, pipelineName)
				assert.TracesFromNamespaceDelivered(t, backend, ns)
				assert.TracePipelineSelfMonitorIsHealthy(t, suite.K8sClient, pipelineName)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			labels := []string{suite.LabelSelfMonitor, tc.component, suite.LabelHealthy}

			var opts []kubeprep.Option
			if isFluentBit(tc.component) {
				opts = append(opts, kubeprep.WithOverrideFIPSMode(false))
			}

			suite.SetupTestWithOptions(t, labels, opts...)

			pipelineName := fmt.Sprintf("selfmonitor-%s", tc.name)

			var (
				uniquePrefix = unique.Prefix(tc.name)
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			backend := kitbackend.New(backendNs, signalTypeForComponent(tc.component))
			pipeline := buildPipeline(tc.component, pipelineName, genNs, backend)

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				pipeline,
			}
			resources = append(resources, tc.generator(genNs)...)
			resources = append(resources, backend.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backend)
			assert.DeploymentReady(t, kitkyma.SelfMonitorName)

			FIPSModeEnabled, err := isFIPSModeEnabled(t)
			Expect(err).ToNot(HaveOccurred())

			if FIPSModeEnabled {
				assert.DeploymentHasImage(t, kitkyma.SelfMonitorName, names.SelfMonitorContainerName, testkit.SelfMonitorFIPSImage)
			} else {
				assert.DeploymentHasImage(t, kitkyma.SelfMonitorName, names.SelfMonitorContainerName, testkit.SelfMonitorImage)
			}

			tc.assert(t, genNs, backend, pipeline.GetName())
		})
	}
}

func isFIPSModeEnabled(t *testing.T) (bool, error) {
	const (
		managerContainerName = "manager"
		fipsEnvVarName       = "KYMA_FIPS_MODE_ENABLED"
	)

	var deployment appsv1.Deployment

	err := suite.K8sClient.Get(t.Context(), kitkyma.TelemetryManagerName, &deployment)
	if err != nil {
		return false, fmt.Errorf("failed to get manager deployment: %w", err)
	}

	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == managerContainerName {
			for _, env := range container.Env {
				if env.Name == fipsEnvVarName && env.Value == "true" {
					return true, nil
				}
			}

			return false, nil
		}
	}

	return false, fmt.Errorf("manager container not found in manager deployment")
}
