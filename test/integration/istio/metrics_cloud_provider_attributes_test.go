//go:build istio

package istio

import (
	"fmt"
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelIntegration), Label(suite.LabelMetrics), Label(suite.LabelSetB), Ordered, func() {
	Context("When metric pipelines with cloud provider resources metrics exist", Ordered, func() {
		var (
			mockNs = suite.ID()

			backendName  = suite.IDWithSuffix("resource-metrics")
			pipelineName = suite.IDWithSuffix("resource-metrics")
			backendURL   string

			DeploymentName = suite.IDWithSuffix("deployment")
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())
			objs = append(objs, kitk8s.NewConfigMap("shoot-info", "kube-system").WithData("shootName", "kyma-telemetry").WithData("provider", "k3d").WithLabel(kitk8s.PersistentLabelName, "true").K8sObject())

			backend := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendName))
			objs = append(objs, backend.K8sObjects()...)
			backendURL = backend.ExportURL(suite.ProxyClient)

			pipeline := testutils.NewMetricPipelineBuilder().
				WithName(pipelineName).
				WithRuntimeInput(true).
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
			objs = append(objs, &pipeline)

			metricProducer := prommetricgen.New(mockNs)

			objs = append(objs, []client.Object{
				metricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
				metricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
			}...)

			podSpec := telemetrygen.PodSpec(telemetrygen.SignalTypeMetrics)

			objs = append(objs, []client.Object{
				kitk8s.NewDeployment(DeploymentName, mockNs).WithPodSpec(podSpec).WithLabel("name", DeploymentName).K8sObject(),
			}...)

			return objs
		}

		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have healthy pipelines", func() {
			assert.MetricPipelineHealthy(suite.Ctx, suite.K8sClient, pipelineName)
		})

		It("Ensures the metric gateway deployment is ready", func() {
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, kitkyma.MetricGatewayName)
		})

		It("Should have metrics backends running", func() {
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: backendName, Namespace: mockNs})
			assert.ServiceReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: backendName, Namespace: mockNs})
		})

		It("should have workloads created properly", func() {
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: DeploymentName, Namespace: mockNs})
		})

		It("Ensures accessibility of metric agent metrics endpoint", func() {
			agentMetricsURL := suite.ProxyClient.ProxyURLForService(kitkyma.MetricAgentMetricsService.Namespace, kitkyma.MetricAgentMetricsService.Name, "metrics", ports.Metrics)
			assert.EmitsOTelCollectorMetrics(suite.ProxyClient, agentMetricsURL)
		})

		Context("Pipeline A should deliver pod metrics", Ordered, func() {
			It("Should deliver pod metrics with expected cloud resource attributes", func() {
				backendContainsDesiredCloudResourceAttributes(suite.ProxyClient, backendURL, "cloud.region")
				backendContainsDesiredCloudResourceAttributes(suite.ProxyClient, backendURL, "cloud.availability_zone")
				backendContainsDesiredCloudResourceAttributes(suite.ProxyClient, backendURL, "host.type")
				backendContainsDesiredCloudResourceAttributes(suite.ProxyClient, backendURL, "host.arch")
				backendContainsDesiredCloudResourceAttributes(suite.ProxyClient, backendURL, "k8s.cluster.name")
				backendContainsDesiredCloudResourceAttributes(suite.ProxyClient, backendURL, "cloud.provider")
			})
		})
	})
})

func backendContainsDesiredCloudResourceAttributes(proxyClient *apiserverproxy.Client, backendExportURL, attribute string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(backendExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		bodyContent, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(bodyContent).To(HaveFlatMetrics(
			ContainElement(SatisfyAll(
				HaveResourceAttributes(HaveKey(attribute)),
			)),
		))
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed(), fmt.Sprintf("could not find metrics matching resource attribute %s", attribute))
}
