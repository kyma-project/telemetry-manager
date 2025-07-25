package istio

import (
	"fmt"
	"io"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/trafficgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMetricsIstioInput(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelIstio)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
		app1Ns       = uniquePrefix("app-1")
		app2Ns       = uniquePrefix("app-2")
	)

	metricBackend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName("metrics"))
	logBackend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithName("logs"))

	metricPipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		WithOTLPInput(false).
		WithIstioInput(true, testutils.IncludeNamespaces(app1Ns)).
		WithOTLPOutput(testutils.OTLPEndpoint(metricBackend.Endpoint())).
		Build()

	logPipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithApplicationInput(false).
		WithOTLPOutput(testutils.OTLPEndpoint(logBackend.Endpoint())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		kitk8s.NewNamespace(app1Ns, kitk8s.WithIstioInjection()).K8sObject(),
		kitk8s.NewNamespace(app2Ns, kitk8s.WithIstioInjection()).K8sObject(),
		&metricPipeline,
		&logPipeline,
	}
	resources = append(resources, metricBackend.K8sObjects()...)
	resources = append(resources, logBackend.K8sObjects()...)
	resources = append(resources, trafficgen.K8sObjects(app1Ns)...)
	resources = append(resources, trafficgen.K8sObjects(app2Ns)...)
	resources = append(resources, telemetrygen.NewDeployment(genNs, telemetrygen.SignalTypeLogs).K8sObject())

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.DeploymentReady(t, kitkyma.MetricGatewayName)
	assert.DaemonSetReady(t, kitkyma.MetricAgentName)
	assert.BackendReachable(t, metricBackend)
	assert.BackendReachable(t, logBackend)

	Eventually(func(g Gomega) {
		backendURL := metricBackend.ExportURL(suite.ProxyClient)
		resp, err := suite.ProxyClient.Get(backendURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(200))
		bodyContent, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(bodyContent).To(HaveFlatMetrics(
			ContainElement(HaveName(BeElementOf([]string{
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
			}))),
		))
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())

	Eventually(func(g Gomega) {
		backendURL := metricBackend.ExportURL(suite.ProxyClient)
		resp, err := suite.ProxyClient.Get(backendURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(200))
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
					HaveKey(BeElementOf([]string{
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
					})),
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

	assert.MetricsFromNamespaceDelivered(t, metricBackend, app1Ns, []string{
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
	})
	assert.MetricsFromNamespaceNotDelivered(t, metricBackend, app2Ns)

	Consistently(func(g Gomega) {
		backendURL := metricBackend.ExportURL(suite.ProxyClient)
		resp, err := suite.ProxyClient.Get(backendURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(200))
		bodyContent, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(bodyContent).NotTo(HaveFlatMetrics(
			ContainElement(HaveMetricAttributes(HaveKeyWithValue("destination_workload", "telemetry-log-gateway"))),
		))
	}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())

	Consistently(func(g Gomega) {
		backendURL := metricBackend.ExportURL(suite.ProxyClient)
		resp, err := suite.ProxyClient.Get(backendURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(200))
		bodyContent, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(bodyContent).To(HaveFlatMetrics(
			Not(
				ContainElement(HaveName(BeElementOf([]string{"up", "scrape_duration_seconds", "scrape_samples_scraped", "scrape_samples_post_metric_relabeling", "scrape_series_added"})))),
		))
	}, periodic.ConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
}
