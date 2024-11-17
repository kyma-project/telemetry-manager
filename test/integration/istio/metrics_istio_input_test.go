//go:build istio

package istio

import (
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"fmt"
	. "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/trafficgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelIntegration), Ordered, func() {
	// https://istio.io/latest/docs/reference/config/metrics/
	var (
		istioProxyMetricNames = []string{
			"istio_requests_total",
			"istio_request_duration_milliseconds",
			"istio_request_bytes",
			"istio_response_bytes",
			"istio_request_messages_total",
			"istio_response_messages_total",
			"istio_tcp_sent_bytes_total",
			"istio_tcp_received_bytes_total",
			"istio_tcp_connections_opened_total",
			"istio_tcp_connections_closed_total",
		}
		istioProxyMetricAttributes = []string{
			"connection_security_policy",
			"destination_app",
			"destination_canonical_revision",
			"destination_canonical_service",
			"destination_cluster",
			"destination_principal",
			"destination_service",
			"destination_service_name",
			"destination_service_namespace",
			"destination_version",
			"destination_workload",
			"destination_workload_namespace",
			"grpc_response_status",
			"request_protocol",
			"response_code",
			"response_flags",
			"source_app",
			"source_canonical_revision",
			"source_canonical_service",
			"source_cluster",
			"source_principal",
			"source_version",
			"source_workload",
			"source_workload_namespace",
		}
		mockNs           = suite.ID()
		app1Ns           = suite.IDWithSuffix("app-1")
		app2Ns           = suite.IDWithSuffix("app-2")
		pipelineName     = suite.ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject(),
			kitk8s.NewNamespace(app1Ns, kitk8s.WithIstioInjection()).K8sObject(),
			kitk8s.NewNamespace(app2Ns, kitk8s.WithIstioInjection()).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeMetrics)
		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(proxyClient)

		metricPipeline := testutils.NewMetricPipelineBuilder().
			WithName(pipelineName).
			WithOTLPInput(false).
			WithIstioInput(true, testutils.IncludeNamespaces(app1Ns)).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()
		objs = append(objs, &metricPipeline)

		objs = append(objs, trafficgen.K8sObjects(app1Ns)...)
		objs = append(objs, trafficgen.K8sObjects(app2Ns)...)

		return objs
	}

	Context("App with istio-sidecar", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})

			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running metric gateway deployment", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Should have a running metric agent daemonset", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		It("Should have a metrics backend running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Should have a running metric agent daemonset", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		It("Should verify istio proxy metric scraping", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(bodyContent).To(HaveFlatMetrics(
					ContainElement(HaveName(BeElementOf(istioProxyMetricNames))),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should verify istio proxy metric attributes", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(bodyContent).To(HaveFlatMetrics(
					SatisfyAll(
						ContainElement(HaveResourceAttributes(SatisfyAll(
							HaveKeyWithValue("k8s.namespace.name", app1Ns),
							HaveKeyWithValue("k8s.pod.name", "destination"),
							HaveKeyWithValue("k8s.container.name", "istio-proxy"),
							HaveKeyWithValue("service.name", "destination"),
						))),
						ContainElement(HaveMetricAttributes(SatisfyAll(
							HaveKey(BeElementOf(istioProxyMetricAttributes)),
							HaveKeyWithValue("source_workload_namespace", app1Ns),
							HaveKeyWithValue("destination_workload", "destination"),
							HaveKeyWithValue("destination_app", "destination"),
							HaveKeyWithValue("destination_service_name", "destination"),
							HaveKeyWithValue("destination_service", fmt.Sprintf("destination.%s.svc.cluster.local", app1Ns)),
							HaveKeyWithValue("destination_service_namespace", app1Ns),
							HaveKeyWithValue("destination_principal", fmt.Sprintf("spiffe://cluster.local/ns/%s/sa/default", app1Ns)),
							HaveKeyWithValue("source_workload", "source"),
							HaveKeyWithValue("source_principal", fmt.Sprintf("spiffe://cluster.local/ns/%s/sa/default", app1Ns)),
							HaveKeyWithValue("response_code", "200"),
							HaveKeyWithValue("request_protocol", "http"),
							HaveKeyWithValue("connection_security_policy", "mutual_tls"),
						))),
						ContainElement(HaveScopeName(ContainSubstring(InstrumentationScopeIstio))),
					),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should deliver metrics from app-1 namespace", func() {
			assert.MetricsFromNamespaceDelivered(proxyClient, backendExportURL, app1Ns, istioProxyMetricNames)
		})

		It("Should not deliver metrics from app-2 namespace", func() {
			assert.MetricsFromNamespaceNotDelivered(proxyClient, backendExportURL, app2Ns)
		})

		It("Should verify that istio metric with source_workload=telemetry-metric-gateway does not exist", func() {
			verifyMetricIsNotPresent(backendExportURL, "source_workload", "telemetry-telemetry-gateway")
		})
		It("Should verify that istio metric with destination_workload=telemetry-metric-gateway does not exist", func() {
			verifyMetricIsNotPresent(backendExportURL, "destination_workload", "telemetry-metric-gateway")
		})

		It("Ensures no diagnostic metrics are sent to backend", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(bodyContent).To(HaveFlatMetrics(
					Not(
						ContainElement(HaveName(BeElementOf("up", "scrape_duration_seconds", "scrape_samples_scraped", "scrape_samples_post_metric_relabeling", "scrape_series_added")))),
				))
			}, periodic.ConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})

func verifyMetricIsNotPresent(backendUrl, key, value string) {
	Consistently(func(g Gomega) {
		resp, err := proxyClient.Get(backendUrl)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		bodyContent, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(bodyContent).NotTo(HaveFlatMetrics(
			ContainElement(HaveMetricAttributes(HaveKeyWithValue(key, value))),
		))
	}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
}
