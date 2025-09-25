package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
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
		inputBuilder     func(includeNss, excludeNss []string) telemetryv1alpha1.MetricPipelineInput
		generatorBuilder func(ns1, ns2 string) []client.Object
	}{
		{
			label: suite.LabelMetricAgentSetB,
			inputBuilder: func(includeNss, excludeNss []string) telemetryv1alpha1.MetricPipelineInput {
				var opts []testutils.NamespaceSelectorOptions
				if len(includeNss) > 0 {
					opts = append(opts, testutils.IncludeNamespaces(includeNss...))
				}

				if len(excludeNss) > 0 {
					opts = append(opts, testutils.ExcludeNamespaces(excludeNss...))
				}

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
			label: suite.LabelMetricGatewaySetB,
			inputBuilder: func(includeNss, excludeNss []string) telemetryv1alpha1.MetricPipelineInput {
				var opts []testutils.NamespaceSelectorOptions
				if len(includeNss) > 0 {
					opts = append(opts, testutils.IncludeNamespaces(includeNss...))
				}

				if len(excludeNss) > 0 {
					opts = append(opts, testutils.ExcludeNamespaces(excludeNss...))
				}

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
				uniquePrefix        = unique.Prefix(tc.label)
				gen1Ns              = uniquePrefix("gen-1")
				gen2Ns              = uniquePrefix("gen-2")
				backendNs           = uniquePrefix("backend")
				backend1Name        = uniquePrefix("backend-1")
				backend2Name        = uniquePrefix("backend-2")
				includePipelineName = uniquePrefix("include")
				excludePipelineName = uniquePrefix("exclude")
			)

			backend1 := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName(backend1Name))
			backend2 := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName(backend2Name))

			// Include gen1Ns only
			includePipeline := testutils.NewMetricPipelineBuilder().
				WithName(includePipelineName).
				WithInput(tc.inputBuilder([]string{gen1Ns}, nil)).
				WithOTLPOutput(testutils.OTLPEndpoint(backend1.Endpoint())).
				Build()

			// Exclude all namespaces except gen2Ns (gen1Ns and other unrelated namespaces)
			// to avoid implicitly collecting logs from other namespaces
			// and potentially overloading the backend.
			var nsList corev1.NamespaceList

			Expect(suite.K8sClient.List(t.Context(), &nsList)).To(Succeed())

			excludeNss := []string{gen1Ns}

			for _, namespace := range nsList.Items {
				if namespace.Name != gen1Ns && namespace.Name != gen2Ns {
					excludeNss = append(excludeNss, namespace.Name)
				}
			}

			excludePipeline := testutils.NewMetricPipelineBuilder().
				WithName(excludePipelineName).
				WithInput(tc.inputBuilder(nil, excludeNss)).
				WithOTLPOutput(testutils.OTLPEndpoint(backend2.Endpoint())).
				Build()

			resources := []client.Object{
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(gen1Ns).K8sObject(),
				kitk8s.NewNamespace(gen2Ns).K8sObject(),
				&includePipeline,
				&excludePipeline,
			}
			resources = append(resources, tc.generatorBuilder(gen1Ns, gen2Ns)...)
			resources = append(resources, backend1.K8sObjects()...)
			resources = append(resources, backend2.K8sObjects()...)

			t.Cleanup(func() {
				Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
			})
			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backend1)
			assert.BackendReachable(t, backend2)
			assert.DeploymentReady(t, kitkyma.MetricGatewayName)

			if suite.ExpectAgent(tc.label) {
				assert.DaemonSetReady(t, kitkyma.MetricAgentName)
			}

			if suite.ExpectAgent(tc.label) {
				assert.MetricsFromNamespaceDelivered(t, backend1, gen1Ns, runtime.DefaultMetricsNames)
				assert.MetricsFromNamespaceDelivered(t, backend1, gen1Ns, prommetricgen.CustomMetricNames())
				assert.MetricsFromNamespaceDelivered(t, backend2, gen2Ns, runtime.DefaultMetricsNames)
				assert.MetricsFromNamespaceDelivered(t, backend2, gen2Ns, prommetricgen.CustomMetricNames())
			} else {
				assert.MetricsFromNamespaceDelivered(t, backend1, gen1Ns, telemetrygen.MetricNames)
				assert.MetricsFromNamespaceDelivered(t, backend2, gen2Ns, telemetrygen.MetricNames)
			}

			assert.MetricsFromNamespaceNotDelivered(t, backend1, gen2Ns)
			assert.MetricsFromNamespaceNotDelivered(t, backend2, gen1Ns)
		})
	}
}
