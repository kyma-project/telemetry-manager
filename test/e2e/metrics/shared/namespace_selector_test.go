package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/metrics/runtime"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestNamespaceSelector(t *testing.T) {
	tests := []struct {
		label            string
		input            func(opts ...testutils.NamespaceSelectorOptions) telemetryv1alpha1.MetricPipelineInput
		generatorBuilder func(ns1, ns2 string) []client.Object
	}{
		{
			label: suite.LabelMetricAgent,
			input: func(opts ...testutils.NamespaceSelectorOptions) telemetryv1alpha1.MetricPipelineInput {
				return testutils.BuildMetricPipelineAgentInput(true, true, true, opts...)
			},
			generatorBuilder: func(ns1, ns2 string) []client.Object {
				return []client.Object{
					prommetricgen.New(ns1).Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
					prommetricgen.New(ns2).Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
					prommetricgen.New(kitkyma.SystemNamespaceName).Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
				}
			},
		},
		{
			label: suite.LabelMetricGateway,
			input: func(opts ...testutils.NamespaceSelectorOptions) telemetryv1alpha1.MetricPipelineInput {
				return testutils.BuildMetricPipelineOTLPInput(opts...)
			},
			generatorBuilder: func(ns1, ns2 string) []client.Object {
				return []client.Object{
					telemetrygen.NewPod(ns1, telemetrygen.SignalTypeMetrics).K8sObject(),
					telemetrygen.NewPod(ns2, telemetrygen.SignalTypeMetrics).K8sObject(),
					telemetrygen.NewPod(kitkyma.SystemNamespaceName, telemetrygen.SignalTypeMetrics).K8sObject(),
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label)

			var (
				uniquePrefix            = unique.Prefix(tc.label)
				app1Ns                  = uniquePrefix("app1")
				app2Ns                  = uniquePrefix("app2")
				backendNs               = uniquePrefix("backend")
				backend1Name            = uniquePrefix("backend1")
				backend2Name            = uniquePrefix("backend2")
				pipelineNameIncludeApp1 = uniquePrefix("include-app1")
				pipelineNameExcludeApp1 = uniquePrefix("exclude-app1")
			)

			backend1 := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName(backend1Name))
			backend2 := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName(backend2Name))

			pipelineIncludeApp1Ns := testutils.NewMetricPipelineBuilder().
				WithName(pipelineNameIncludeApp1).
				WithInput(tc.input(testutils.IncludeNamespaces(app1Ns))).
				WithOTLPOutput(testutils.OTLPEndpoint(backend1.Endpoint())).
				Build()

			pipelineExcludeApp1Ns := testutils.NewMetricPipelineBuilder().
				WithName(pipelineNameExcludeApp1).
				WithInput(tc.input(testutils.ExcludeNamespaces(app1Ns))).
				WithOTLPOutput(testutils.OTLPEndpoint(backend2.Endpoint())).
				Build()

			resources := []client.Object{
				kitk8s.NewNamespace(app1Ns).K8sObject(),
				kitk8s.NewNamespace(app2Ns).K8sObject(),
				kitk8s.NewNamespace(backendNs).K8sObject(),
				&pipelineIncludeApp1Ns,
				&pipelineExcludeApp1Ns,
			}
			resources = append(resources, tc.generatorBuilder(app1Ns, app2Ns)...)
			resources = append(resources, backend1.K8sObjects()...)
			resources = append(resources, backend2.K8sObjects()...)

			t.Cleanup(func() {
				Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
			})
			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backend1)
			assert.BackendReachable(t, backend2)
			assert.DeploymentReady(t, kitkyma.MetricGatewayName)

			if tc.label == suite.LabelMetricAgent {
				assert.DaemonSetReady(t, kitkyma.MetricAgentName)
			}

			switch tc.label {
			case suite.LabelMetricAgent:
				assert.MetricsFromNamespaceDelivered(t, backend1, app1Ns, runtime.DefaultMetricsNames)
				assert.MetricsFromNamespaceDelivered(t, backend1, app1Ns, prommetricgen.CustomMetricNames())
				assert.MetricsFromNamespaceDelivered(t, backend2, app2Ns, runtime.DefaultMetricsNames)
				assert.MetricsFromNamespaceDelivered(t, backend2, app2Ns, prommetricgen.CustomMetricNames())
			case suite.LabelMetricGateway:
				assert.MetricsFromNamespaceDelivered(t, backend1, app1Ns, telemetrygen.MetricNames)
				assert.MetricsFromNamespaceDelivered(t, backend2, app2Ns, telemetrygen.MetricNames)
			}

			assert.MetricsFromNamespaceNotDelivered(t, backend1, app2Ns)
			assert.MetricsFromNamespaceNotDelivered(t, backend2, app1Ns)
		})
	}
}
