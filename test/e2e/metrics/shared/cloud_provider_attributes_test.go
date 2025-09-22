package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestCloudProviderAttributes(t *testing.T) {
	tests := []struct {
		label            string
		input            telemetryv1alpha1.MetricPipelineInput
		generatorBuilder func(ns string) []client.Object
	}{
		{
			label: suite.LabelMetricAgent,
			input: testutils.BuildMetricPipelineRuntimeInput(),
			generatorBuilder: func(ns string) []client.Object {
				generator := prommetricgen.New(ns)

				return []client.Object{
					generator.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
					generator.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
				}
			},
		},
		{
			label: suite.LabelMetricGateway,
			input: testutils.BuildMetricPipelineOTLPInput(),
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
				uniquePrefix   = unique.Prefix(tc.label)
				pipelineName   = uniquePrefix()
				deploymentName = uniquePrefix("deployment")
				genNs          = uniquePrefix("gen")
				mockNs         = uniquePrefix("mock")
			)

			backend := kitbackend.New(mockNs, kitbackend.SignalTypeMetrics)
			pipeline := testutils.NewMetricPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.input).
				WithRuntimeInputContainerMetrics(true).
				WithRuntimeInputPodMetrics(true).
				WithRuntimeInputNodeMetrics(true).
				WithRuntimeInputVolumeMetrics(true).
				WithRuntimeInputDeploymentMetrics(false).
				WithRuntimeInputStatefulSetMetrics(false).
				WithRuntimeInputDaemonSetMetrics(false).
				WithRuntimeInputJobMetrics(false).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
				Build()

			podSpec := telemetrygen.PodSpec(telemetrygen.SignalTypeMetrics)

			deployment := kitk8s.NewDeployment(deploymentName, mockNs).WithPodSpec(podSpec).WithLabel("name", deploymentName).K8sObject()

			resources := []client.Object{
				kitk8s.NewNamespace(mockNs).K8sObject(),
				kitk8s.NewNamespace(genNs).K8sObject(),
				&pipeline,
				deployment,
			}
			resources = append(resources, tc.generatorBuilder(genNs)...)
			resources = append(resources, backend.K8sObjects()...)

			t.Cleanup(func() {
				Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
			})
			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backend)
			assert.DeploymentReady(t, kitkyma.MetricGatewayName)

			if tc.label == suite.LabelMetricAgent {
				assert.DaemonSetReady(t, kitkyma.MetricAgentName)
			}

			assert.MetricPipelineHealthy(t, pipelineName)

			assert.DeploymentReady(t, types.NamespacedName{Name: deploymentName, Namespace: mockNs})

			if tc.label == suite.LabelMetricAgent {
				agentMetricsURL := suite.ProxyClient.ProxyURLForService(kitkyma.MetricAgentMetricsService.Namespace, kitkyma.MetricAgentMetricsService.Name, "metrics", ports.Metrics)
				assert.EmitsOTelCollectorMetrics(t, agentMetricsURL)
			}

			assert.BackendDataEventuallyMatches(t, backend,
				HaveFlatMetrics(
					ContainElement(HaveResourceAttributes(SatisfyAll(
						HaveKey("cloud.region"),
						HaveKey("cloud.availability_zone"),
						HaveKey("host.type"),
						HaveKey("host.arch"),
						HaveKey("k8s.cluster.name"),
						HaveKey("cloud.provider"),
					))),
				), assert.WithOptionalDescription("Could not find metrics matching resource attributes"),
			)
		})
	}
}
