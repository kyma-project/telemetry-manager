//go:build e2e

package metrics

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/metrics/runtime"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelMetrics), Label(LabelSetB), Ordered, func() {
	var (
		backendOnlyNodeMetricsEnabledName  = IDWithSuffix("node-metrics")
		pipelineOnlyNodeMetricsEnabledName = IDWithSuffix("node-metrics")
	)

	type testcase struct {
		pipeline  *testutils.MetricPipelineBuilder
		name      string
		namespace string
	}
	testcases := []testcase{
		{
			namespace: IDWithSuffix("exlude-ns"),
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName(pipelineOnlyNodeMetricsEnabledName).
				WithRuntimeInput(true,
					testutils.ExcludeNamespaces(IDWithSuffix("exlude-ns")),
				).
				WithRuntimeInputNodeMetrics(true).
				WithRuntimeInputPodMetrics(false).
				WithRuntimeInputContainerMetrics(false).
				WithRuntimeInputVolumeMetrics(false),
			name: "exclude",
		},
		{
			namespace: IDWithSuffix("include-ns"),
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName(pipelineOnlyNodeMetricsEnabledName).
				WithRuntimeInput(true,
					testutils.IncludeNamespaces(IDWithSuffix("include-ns")),
				).
				WithRuntimeInputNodeMetrics(true).
				WithRuntimeInputPodMetrics(false).
				WithRuntimeInputContainerMetrics(false).
				WithRuntimeInputVolumeMetrics(false),
			name: "include",
		},
	}

	for _, tt := range testcases {
		Context("When metric pipelines with node metrics enabled and an "+tt.name+" namespace selector exist", Ordered, func() {
			var (
				backendOnlyNodeMetricsEnabledURL string
			)

			makeResources := func() []client.Object {
				var objs []client.Object
				objs = append(objs, kitk8s.NewNamespace(tt.namespace).K8sObject())

				backendOnlyNodeMetricsEnabled := backend.New(tt.namespace, backend.SignalTypeMetrics, backend.WithName(backendOnlyNodeMetricsEnabledName))
				objs = append(objs, backendOnlyNodeMetricsEnabled.K8sObjects()...)
				backendOnlyNodeMetricsEnabledURL = backendOnlyNodeMetricsEnabled.ExportURL(ProxyClient)

				pipeline := tt.pipeline.WithOTLPOutput(testutils.OTLPEndpoint(backendOnlyNodeMetricsEnabled.Endpoint())).Build()

				objs = append(objs, &pipeline)

				metricProducer := prommetricgen.New(tt.namespace)

				objs = append(objs, []client.Object{
					metricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
					metricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
				}...)

				return objs
			}

			BeforeAll(func() {
				k8sObjects := makeResources()

				DeferCleanup(func() {
					Expect(kitk8s.DeleteObjects(Ctx, K8sClient, k8sObjects...)).Should(Succeed())
				})
				Expect(kitk8s.CreateObjects(Ctx, K8sClient, k8sObjects...)).Should(Succeed())
			})

			It("Should have healthy pipelines", func() {
				assert.MetricPipelineHealthy(Ctx, K8sClient, pipelineOnlyNodeMetricsEnabledName)
			})

			It("Ensures the metric gateway deployment is ready", func() {
				assert.DeploymentReady(Ctx, K8sClient, kitkyma.MetricGatewayName)
			})

			It("Ensures the metric agent daemonset is ready", func() {
				assert.DaemonSetReady(Ctx, K8sClient, kitkyma.MetricAgentName)
			})

			It("Should have metrics backends running", func() {
				assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Name: backendOnlyNodeMetricsEnabledName, Namespace: tt.namespace})
				assert.ServiceReady(Ctx, K8sClient, types.NamespacedName{Name: backendOnlyNodeMetricsEnabledName, Namespace: tt.namespace})

			})

			Context("Runtime node metrics", func() {
				It("Should deliver runtime node metrics to node-metrics backend even though node metrics do not exist on namespace level", func() {
					Eventually(func(g Gomega) {
						resp, err := ProxyClient.Get(backendOnlyNodeMetricsEnabledURL)
						g.Expect(err).NotTo(HaveOccurred())
						g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

						g.Expect(resp).To(HaveHTTPBody(
							HaveFlatMetrics(HaveUniqueNamesForRuntimeScope(ConsistOf(runtime.NodeMetricsNames))),
						))
					}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
				})

				It("Should have expected resource attributes in runtime node metrics", func() {
					Eventually(func(g Gomega) {
						resp, err := ProxyClient.Get(backendOnlyNodeMetricsEnabledURL)
						g.Expect(err).NotTo(HaveOccurred())
						g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

						g.Expect(resp).To(HaveHTTPBody(
							HaveFlatMetrics(ContainElement(HaveResourceAttributes(HaveKeys(ConsistOf(runtime.NodeMetricsResourceAttributes))))),
						))
					}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
				})
			})
		})
	}
})
