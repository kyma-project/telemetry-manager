package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/metrics/runtime"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestSinglePipelineV1Alpha1(t *testing.T) {
	tests := []struct {
		label            string
		input            telemetryv1alpha1.MetricPipelineInput
		generatorBuilder func(ns string) []client.Object
	}{
		{
			label: suite.LabelMetricAgentSetC,
			input: telemetryv1alpha1.MetricPipelineInput{
				Runtime: &telemetryv1alpha1.MetricPipelineRuntimeInput{
					Enabled: ptr.To(true),
				},
			},
			generatorBuilder: func(ns string) []client.Object {
				generator := prommetricgen.New(ns)

				return []client.Object{
					generator.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
					generator.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
				}
			},
		},
		{
			label: suite.LabelMetricGatewaySetC,
			input: telemetryv1alpha1.MetricPipelineInput{
				OTLP: &telemetryv1alpha1.OTLPInput{},
			},
			generatorBuilder: func(ns string) []client.Object {
				return []client.Object{
					telemetrygen.NewPod(ns, telemetrygen.SignalTypeMetrics).K8sObject(),
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label)

			var (
				uniquePrefix = unique.Prefix(tc.label)
				pipelineName = uniquePrefix()
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)
			pipeline := telemetryv1alpha1.MetricPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: pipelineName,
				},
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: tc.input,
					Output: telemetryv1alpha1.MetricPipelineOutput{
						OTLP: &telemetryv1alpha1.OTLPOutput{
							Endpoint: telemetryv1alpha1.ValueType{
								Value: backend.EndpointHTTP(),
							},
						},
					},
				},
			}

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				&pipeline,
			}
			resources = append(resources, backend.K8sObjects()...)
			resources = append(resources, tc.generatorBuilder(genNs)...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backend)
			assert.DeploymentReady(t, kitkyma.MetricGatewayName)

			if tc.label == suite.LabelLogAgent {
				assert.DaemonSetReady(t, kitkyma.MetricAgentName)
			}

			assert.MetricPipelineHealthy(t, pipelineName)

			if suite.ExpectAgent(tc.label) {
				assert.MetricsFromNamespaceDelivered(t, backend, genNs, runtime.DefaultMetricsNames)

				agentMetricsURL := suite.ProxyClient.ProxyURLForService(kitkyma.MetricAgentMetricsService.Namespace, kitkyma.MetricAgentMetricsService.Name, "metrics", ports.Metrics)
				assert.EmitsOTelCollectorMetrics(t, agentMetricsURL)
			} else {
				assert.MetricsFromNamespaceDelivered(t, backend, genNs, telemetrygen.MetricNames)

				gatewayMetricsURL := suite.ProxyClient.ProxyURLForService(kitkyma.MetricGatewayMetricsService.Namespace, kitkyma.MetricGatewayMetricsService.Name, "metrics", ports.Metrics)
				assert.EmitsOTelCollectorMetrics(t, gatewayMetricsURL)
			}
		})
	}
}
