package misc

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
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

var (
	managerLabel          = map[string]string{"my-manager-label": "manager-value"}
	managerAnnotation     = map[string]string{"my-manager-annotation": "manager-annotation-value"}
	managerPodLabel       = map[string]string{"my-manager-pod-label": "manager-pod-value"}
	managerPodAnnotation  = map[string]string{"my-manager-pod-annotation": "manager-pod-annotation-value"}
	workloadLabel         = map[string]string{"my-workload-label": "workload-value"}
	workloadAnnotation    = map[string]string{"my-workload-annotation": "workload-annotation-value"}
	workloadPodLabel      = map[string]string{"my-pod-label": "pod-value"}
	workloadPodAnnotation = map[string]string{"my-pod-annotation": "pod-annotation-value"}
)

func TestLabelAnnotation(t *testing.T) {
	tests := []struct {
		labelPrefix kitbackend.SignalType
		pipeline    func(includeNs string, backend *kitbackend.Backend) client.Object
		generator   func(ns string) []client.Object
		assert      func(t *testing.T, ns string, backend *kitbackend.Backend, pipelineName string)
	}{
		{
			labelPrefix: kitbackend.SignalTypeLogsOTel,
			pipeline: func(includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewLogPipelineBuilder().
					WithName("custom-otel-log-agent").
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

				// Gateway should have workload labels/annotations
				assert.DeploymentHasLabel(t, kitkyma.LogGatewayName, workloadLabel)
				assert.DeploymentHasAnnotation(t, kitkyma.LogGatewayName, workloadAnnotation)

				var gwSelector = client.ListOptions{
					LabelSelector: labels.SelectorFromSet(map[string]string{"app.kubernetes.io/name": "telemetry-log-gateway"}),
					Namespace:     kitkyma.SystemNamespaceName,
				}
				// Gateway pods should have both workload and pod-specific labels/annotations
				assert.PodsHaveAnnotation(t, gwSelector, workloadAnnotation)
				assert.PodsHaveAnnotation(t, gwSelector, workloadPodAnnotation)
				assert.PodsHaveLabel(t, gwSelector, workloadLabel)
				assert.PodsHaveLabel(t, gwSelector, workloadPodLabel)

				// Agent should have workload labels/annotations
				assert.DaemonSetHasLabel(t, kitkyma.LogAgentName, workloadLabel)
				assert.DaemonSetHasAnnotation(t, kitkyma.LogAgentName, workloadAnnotation)

				var agentSelector = client.ListOptions{
					LabelSelector: labels.SelectorFromSet(map[string]string{"app.kubernetes.io/name": "telemetry-log-agent"}),
					Namespace:     kitkyma.SystemNamespaceName,
				}
				// Agent pods should have both workload and pod-specific labels/annotations
				assert.PodsHaveAnnotation(t, agentSelector, workloadAnnotation)
				assert.PodsHaveAnnotation(t, agentSelector, workloadPodAnnotation)
				assert.PodsHaveLabel(t, agentSelector, workloadLabel)
				assert.PodsHaveLabel(t, agentSelector, workloadPodLabel)
			},
		},
		{
			labelPrefix: kitbackend.SignalTypeLogsFluentBit,
			pipeline: func(includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewLogPipelineBuilder().
					WithName("custom-fluent-bit").
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

				// FluentBit agent should have workload labels/annotations
				assert.DaemonSetHasLabel(t, kitkyma.FluentBitDaemonSetName, workloadLabel)
				assert.DaemonSetHasAnnotation(t, kitkyma.FluentBitDaemonSetName, workloadAnnotation)

				var selector = client.ListOptions{
					LabelSelector: labels.SelectorFromSet(map[string]string{"app.kubernetes.io/name": "fluent-bit"}),
					Namespace:     kitkyma.SystemNamespaceName,
				}
				// FluentBit pods should have both workload and pod-specific labels/annotations
				assert.PodsHaveAnnotation(t, selector, workloadAnnotation)
				assert.PodsHaveAnnotation(t, selector, workloadPodAnnotation)
				assert.PodsHaveLabel(t, selector, workloadLabel)
				assert.PodsHaveLabel(t, selector, workloadPodLabel)
			},
		},

		{
			labelPrefix: kitbackend.SignalTypeMetrics,
			pipeline: func(includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewMetricPipelineBuilder().
					WithName("custom-metric-agent").
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

				// Gateway should have workload labels/annotations
				assert.DeploymentHasLabel(t, kitkyma.MetricGatewayName, workloadLabel)
				assert.DeploymentHasAnnotation(t, kitkyma.MetricGatewayName, workloadAnnotation)

				var gwSelector = client.ListOptions{
					LabelSelector: labels.SelectorFromSet(map[string]string{"app.kubernetes.io/name": "telemetry-metric-gateway"}),
					Namespace:     kitkyma.SystemNamespaceName,
				}
				// Gateway pods should have both workload and pod-specific labels/annotations
				assert.PodsHaveAnnotation(t, gwSelector, workloadAnnotation)
				assert.PodsHaveAnnotation(t, gwSelector, workloadPodAnnotation)
				assert.PodsHaveLabel(t, gwSelector, workloadLabel)
				assert.PodsHaveLabel(t, gwSelector, workloadPodLabel)

				// Agent should have workload labels/annotations
				assert.DaemonSetHasLabel(t, kitkyma.MetricAgentName, workloadLabel)
				assert.DaemonSetHasAnnotation(t, kitkyma.MetricAgentName, workloadAnnotation)

				var agentSelector = client.ListOptions{
					LabelSelector: labels.SelectorFromSet(map[string]string{"app.kubernetes.io/name": "telemetry-metric-agent"}),
					Namespace:     kitkyma.SystemNamespaceName,
				}
				// Agent pods should have both workload and pod-specific labels/annotations
				assert.PodsHaveAnnotation(t, agentSelector, workloadAnnotation)
				assert.PodsHaveAnnotation(t, agentSelector, workloadPodAnnotation)
				assert.PodsHaveLabel(t, agentSelector, workloadLabel)
				assert.PodsHaveLabel(t, agentSelector, workloadPodLabel)
			},
		},
		{
			labelPrefix: kitbackend.SignalTypeTraces,
			pipeline: func(includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewTracePipelineBuilder().
					WithName("custom-trace").
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

				// Gateway should have workload labels/annotations
				assert.DeploymentHasLabel(t, kitkyma.TraceGatewayName, workloadLabel)
				assert.DeploymentHasAnnotation(t, kitkyma.TraceGatewayName, workloadAnnotation)

				var selector = client.ListOptions{
					LabelSelector: labels.SelectorFromSet(map[string]string{"app.kubernetes.io/name": "telemetry-trace-gateway"}),
					Namespace:     kitkyma.SystemNamespaceName,
				}
				// Gateway pods should have both workload and pod-specific labels/annotations
				assert.PodsHaveAnnotation(t, selector, workloadAnnotation)
				assert.PodsHaveAnnotation(t, selector, workloadPodAnnotation)
				assert.PodsHaveLabel(t, selector, workloadLabel)
				assert.PodsHaveLabel(t, selector, workloadPodLabel)
			},
		},
	}

	for _, tc := range tests {
		t.Run(string(tc.labelPrefix), func(t *testing.T) {
			// Use SetupTestWithOptions with custom helm values for workload and pod labels/annotations
			suite.SetupTestWithOptions(t,
				[]string{suite.LabelCustomLabelAnnotation},
				kubeprep.WithOverrideFIPSMode(false),
				kubeprep.WithHelmValues(
					"manager.labels.my-manager-label=manager-value",
					"manager.annotations.my-manager-annotation=manager-annotation-value",
					"manager.pod.labels.my-manager-pod-label=manager-pod-value",
					"manager.pod.annotations.my-manager-pod-annotation=manager-pod-annotation-value",
					"managedResources.workload.labels.my-workload-label=workload-value",
					"managedResources.workload.annotations.my-workload-annotation=workload-annotation-value",
					"managedResources.workload.pod.labels.my-pod-label=pod-value",
					"managedResources.workload.pod.annotations.my-pod-annotation=pod-annotation-value",
				))

			var (
				uniquePrefix = unique.Prefix(string(tc.labelPrefix))
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			backend := kitbackend.New(backendNs, tc.labelPrefix)
			pipeline := tc.pipeline(genNs, backend)
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

			// Telemetry Manager should have manager labels/annotations
			assert.DeploymentHasLabel(t, kitkyma.TelemetryManagerName, managerLabel)
			assert.DeploymentHasAnnotation(t, kitkyma.TelemetryManagerName, managerAnnotation)

			var managerSelector = client.ListOptions{
				LabelSelector: labels.SelectorFromSet(map[string]string{"app.kubernetes.io/name": "telemetry-manager"}),
				Namespace:     kitkyma.SystemNamespaceName,
			}
			// Telemetry Manager pods should have both manager and pod-specific labels/annotations
			assert.PodsHaveLabel(t, managerSelector, managerLabel)
			assert.PodsHaveLabel(t, managerSelector, managerPodLabel)
			assert.PodsHaveAnnotation(t, managerSelector, managerAnnotation)
			assert.PodsHaveAnnotation(t, managerSelector, managerPodAnnotation)
			// Telemetry Manager pods should have default pod annotations
			assert.PodsHaveAnnotation(t, managerSelector, map[string]string{"kubectl.kubernetes.io/default-container": "manager"})
			assert.PodsHaveAnnotation(t, managerSelector, map[string]string{"sidecar.istio.io/inject": "false"})

			tc.assert(t, genNs, backend, pipeline.GetName())
		})
	}
}
