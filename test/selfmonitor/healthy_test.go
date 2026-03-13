package selfmonitor

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestHealthy(t *testing.T) {
	tests := []struct {
		name      string
		component string
		pipeline  func(pipelineName, includeNs string, backend *kitbackend.Backend) client.Object
		generator func(ns string) []client.Object
		assert    func(t *testing.T, ns string, backend *kitbackend.Backend, pipelineName string)
	}{
		{
			name:      "log-agent",
			component: suite.LabelLogAgent,
			pipeline: func(pipelineName, includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewLogPipelineBuilder().
					WithName(pipelineName).
					WithInput(testutils.BuildLogPipelineRuntimeInput(testutils.IncludeNamespaces(includeNs))).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
					Build()

				return &p
			},
			generator: func(ns string) []client.Object {
				return []client.Object{
					stdoutloggen.NewDeployment(ns).K8sObject(),
				}
			},
			assert: func(t *testing.T, ns string, backend *kitbackend.Backend, pipelineName string) {
				assert.DeploymentReady(t, kitkyma.LogGatewayName)
				assert.DaemonSetReady(t, kitkyma.LogAgentName)
				assert.OTelLogPipelineHealthy(t, pipelineName)
				assert.OTelLogsFromNamespaceDelivered(t, backend, ns)
				assert.LogPipelineSelfMonitorIsHealthy(t, suite.K8sClient, pipelineName)
			},
		},
		{
			name:      "log-gateway",
			component: suite.LabelLogGateway,
			pipeline: func(pipelineName, includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewLogPipelineBuilder().
					WithName(pipelineName).
					WithInput(testutils.BuildLogPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
					Build()

				return &p
			},
			generator: func(ns string) []client.Object {
				return []client.Object{
					telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeLogs).K8sObject(),
				}
			},
			assert: func(t *testing.T, ns string, backend *kitbackend.Backend, pipelineName string) {
				assert.DeploymentReady(t, kitkyma.LogGatewayName)
				assert.OTelLogPipelineHealthy(t, pipelineName)
				assert.OTelLogsFromNamespaceDelivered(t, backend, ns)
				assert.LogPipelineSelfMonitorIsHealthy(t, suite.K8sClient, pipelineName)
			},
		},
		{
			name:      "fluent-bit",
			component: suite.LabelFluentBit,
			pipeline: func(pipelineName, includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewLogPipelineBuilder().
					WithName(pipelineName).
					WithRuntimeInput(true, testutils.IncludeNamespaces(includeNs)).
					WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
					Build()

				return &p
			},
			generator: func(ns string) []client.Object {
				return []client.Object{
					stdoutloggen.NewDeployment(ns).K8sObject(),
				}
			},
			assert: func(t *testing.T, ns string, backend *kitbackend.Backend, pipelineName string) {
				assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
				assert.FluentBitLogPipelineHealthy(t, pipelineName)
				assert.FluentBitLogsFromNamespaceDelivered(t, backend, ns)
				assert.LogPipelineSelfMonitorIsHealthy(t, suite.K8sClient, pipelineName)
			},
		},
		{
			name:      "metric-gateway",
			component: suite.LabelMetricGateway,
			pipeline: func(pipelineName, includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewMetricPipelineBuilder().
					WithName(pipelineName).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
					Build()

				return &p
			},
			generator: func(ns string) []client.Object {
				return []client.Object{
					telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeMetrics).K8sObject(),
				}
			},
			assert: func(t *testing.T, ns string, backend *kitbackend.Backend, pipelineName string) {
				assert.DeploymentReady(t, kitkyma.MetricGatewayName)
				assert.MetricPipelineHealthy(t, pipelineName)
				assert.MetricsFromNamespaceDelivered(t, backend, ns, telemetrygen.MetricNames)
				assert.MetricPipelineSelfMonitorIsHealthy(t, suite.K8sClient, pipelineName)
			},
		},
		{
			name:      "metric-agent",
			component: suite.LabelMetricAgent,
			pipeline: func(pipelineName, includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewMetricPipelineBuilder().
					WithName(pipelineName).
					WithPrometheusInput(true, testutils.IncludeNamespaces(includeNs)).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
					Build()

				return &p
			},
			generator: func(ns string) []client.Object {
				metricProducer := prommetricgen.New(ns)

				return []client.Object{
					metricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
					metricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
				}
			},
			assert: func(t *testing.T, ns string, backend *kitbackend.Backend, pipelineName string) {
				assert.DeploymentReady(t, kitkyma.MetricGatewayName)
				assert.DaemonSetReady(t, kitkyma.MetricAgentName)
				assert.MetricPipelineHealthy(t, pipelineName)
				assert.MetricsFromNamespaceDelivered(t, backend, ns, prommetricgen.CustomMetricNames())
				assert.MetricPipelineSelfMonitorIsHealthy(t, suite.K8sClient, pipelineName)
			},
		},
		{
			name:      "traces",
			component: suite.LabelTraces,
			pipeline: func(pipelineName, includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewTracePipelineBuilder().
					WithName(pipelineName).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
					Build()

				return &p
			},
			generator: func(ns string) []client.Object {
				return []client.Object{
					telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeTraces).K8sObject(),
				}
			},
			assert: func(t *testing.T, ns string, backend *kitbackend.Backend, pipelineName string) {
				assert.DeploymentReady(t, kitkyma.TraceGatewayName)
				assert.TracePipelineHealthy(t, pipelineName)
				assert.TracesFromNamespaceDelivered(t, backend, ns)
				assert.TracePipelineSelfMonitorIsHealthy(t, suite.K8sClient, pipelineName)
			},
		},
	}

	// Tests run once per test case. FIPS mode is determined by environment (FIPS_IMAGE_AVAILABLE).
	// FluentBit tests always run in no-FIPS mode via WithOverrideFIPSMode(false).
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Labels: selfmonitor + component + scenario
			labels := []string{
				suite.LabelSelfMonitor,
				tc.component,
				suite.LabelHealthy,
			}

			// FluentBit doesn't support FIPS mode
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
			pipeline := tc.pipeline(pipelineName, genNs, backend)
			generator := tc.generator(genNs)

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				pipeline,
			}
			resources = append(resources, generator...)
			resources = append(resources, backend.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backend)
			assert.DeploymentReady(t, kitkyma.SelfMonitorName)

			FIPSModeEnabled, err := isFIPSModeEnabled(t)
			Expect(err).ToNot(HaveOccurred())

			if FIPSModeEnabled {
				// assert that the Self-Monitor image is the prometheus-fips image when FIPS mode is enabled
				assert.DeploymentHasImage(t, kitkyma.SelfMonitorName, names.SelfMonitorContainerName, testkit.SelfMonitorFIPSImage)
			} else {
				// assert that the Self-Monitor image is the regular telemetry-self-monitor image when FIPS mode is not enabled
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
