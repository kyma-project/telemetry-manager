//go:build e2e

package e2e

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
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), Label(suite.LabelSetB), Ordered, func() {
	var (
		backendOnlyNodeMetricsEnabledName  = suite.IDWithSuffix("node-metrics")
		pipelineOnlyNodeMetricsEnabledName = suite.IDWithSuffix("node-metrics")
	)

	type testcase struct {
		pipeline  *testutils.MetricPipelineBuilder
		name      string
		namespace string
	}
	testcases := []testcase{
		{
			namespace: suite.IDWithSuffix("exlude-ns"),
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName(pipelineOnlyNodeMetricsEnabledName).
				WithRuntimeInput(true,
					testutils.ExcludeNamespaces(suite.IDWithSuffix("exlude-ns")),
				).
				WithRuntimeInputNodeMetrics(true).
				WithRuntimeInputPodMetrics(false).
				WithRuntimeInputContainerMetrics(false).
				WithRuntimeInputVolumeMetrics(false),
			name: "exclude",
		},
		{
			namespace: suite.IDWithSuffix("include-ns"),
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName(pipelineOnlyNodeMetricsEnabledName).
				WithRuntimeInput(true,
					testutils.IncludeNamespaces(suite.IDWithSuffix("include-ns")),
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
				backendOnlyNodeMetricsEnabledURL = backendOnlyNodeMetricsEnabled.ExportURL(proxyClient)

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
					Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
				})
				Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})

			It("Should have healthy pipelines", func() {
				assert.MetricPipelineHealthy(ctx, k8sClient, pipelineOnlyNodeMetricsEnabledName)
			})

			It("Ensures the metric gateway deployment is ready", func() {
				assert.DeploymentReady(ctx, k8sClient, kitkyma.MetricGatewayName)
			})

			It("Ensures the metric agent daemonset is ready", func() {
				assert.DaemonSetReady(ctx, k8sClient, kitkyma.MetricAgentName)
			})

			It("Should have metrics backends running", func() {
				assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backendOnlyNodeMetricsEnabledName, Namespace: tt.namespace})
				assert.ServiceReady(ctx, k8sClient, types.NamespacedName{Name: backendOnlyNodeMetricsEnabledName, Namespace: tt.namespace})

			})

			Context("Runtime node metrics", func() {
				It("Should deliver runtime node metrics to node-metrics backend even though node metrics do not exist on namespace level", func() {
					Eventually(func(g Gomega) {
						resp, err := proxyClient.Get(backendOnlyNodeMetricsEnabledURL)
						g.Expect(err).NotTo(HaveOccurred())
						g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

						g.Expect(resp).To(HaveHTTPBody(
							HaveFlatMetrics(HaveUniqueNamesForRuntimeScope(ConsistOf(runtime.NodeMetricsNames))),
						))
					}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
				})

				It("Should have expected resource attributes in runtime node metrics", func() {
					Eventually(func(g Gomega) {
						resp, err := proxyClient.Get(backendOnlyNodeMetricsEnabledURL)
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
