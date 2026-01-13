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
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

var label = map[string]string{"my-meta-label": "foo"}
var annotation = map[string]string{"my-meta-annotation": "bar"}

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
				assert.DeploymentHasLabel(t, kitkyma.LogGatewayName, label)
				assert.DeploymentHasAnnotation(t, kitkyma.LogGatewayName, annotation)

				var gwSelector = client.ListOptions{
					LabelSelector: labels.SelectorFromSet(map[string]string{"app.kubernetes.io/name": "telemetry-log-gateway"}),
					Namespace:     kitkyma.SystemNamespaceName,
				}
				assert.PodsHaveAnnotation(t, gwSelector, annotation)
				assert.PodsHaveLabel(t, gwSelector, label)

				assert.DaemonSetHasLabel(t, kitkyma.LogAgentName, label)
				assert.DaemonSetHasAnnotation(t, kitkyma.LogAgentName, annotation)

				var agentSelector = client.ListOptions{
					LabelSelector: labels.SelectorFromSet(map[string]string{"app.kubernetes.io/name": "telemetry-log-agent"}),
					Namespace:     kitkyma.SystemNamespaceName,
				}
				assert.PodsHaveAnnotation(t, agentSelector, annotation)
				assert.PodsHaveLabel(t, agentSelector, label)
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
				assert.DaemonSetHasLabel(t, kitkyma.FluentBitDaemonSetName, label)
				assert.DaemonSetHasAnnotation(t, kitkyma.FluentBitDaemonSetName, annotation)

				var selector = client.ListOptions{
					LabelSelector: labels.SelectorFromSet(map[string]string{"app.kubernetes.io/name": "fluent-bit"}),
					Namespace:     kitkyma.SystemNamespaceName,
				}
				assert.PodsHaveAnnotation(t, selector, annotation)
				assert.PodsHaveLabel(t, selector, label)
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

				assert.DeploymentHasLabel(t, kitkyma.MetricGatewayName, label)
				assert.DeploymentHasAnnotation(t, kitkyma.MetricGatewayName, annotation)

				var gwSelector = client.ListOptions{
					LabelSelector: labels.SelectorFromSet(map[string]string{"app.kubernetes.io/name": "telemetry-metric-gateway"}),
					Namespace:     kitkyma.SystemNamespaceName,
				}
				assert.PodsHaveAnnotation(t, gwSelector, annotation)
				assert.PodsHaveLabel(t, gwSelector, label)

				assert.DaemonSetHasLabel(t, kitkyma.MetricAgentName, label)
				assert.DaemonSetHasAnnotation(t, kitkyma.MetricAgentName, annotation)

				var agentSelector = client.ListOptions{
					LabelSelector: labels.SelectorFromSet(map[string]string{"app.kubernetes.io/name": "telemetry-metric-agent"}),
					Namespace:     kitkyma.SystemNamespaceName,
				}
				assert.PodsHaveAnnotation(t, agentSelector, annotation)
				assert.PodsHaveLabel(t, agentSelector, label)
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

				assert.DeploymentHasLabel(t, kitkyma.TraceGatewayName, label)
				assert.DeploymentHasAnnotation(t, kitkyma.TraceGatewayName, annotation)

				var selector = client.ListOptions{
					LabelSelector: labels.SelectorFromSet(map[string]string{"app.kubernetes.io/name": "telemetry-trace-gateway"}),
					Namespace:     kitkyma.SystemNamespaceName,
				}
				assert.PodsHaveAnnotation(t, selector, annotation)
				assert.PodsHaveLabel(t, selector, label)
			},
		},
	}

	for _, tc := range tests {
		t.Run(string(tc.labelPrefix), func(t *testing.T) {
			suite.RegisterTestCase(t, suite.LabelCustomLabelAnnotation)

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

			tc.assert(t, genNs, backend, pipeline.GetName())
		})
	}
}
