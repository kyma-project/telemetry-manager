package istio

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/trafficgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMetricsIstioInput(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelGardener, suite.LabelIstio)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
		app1Ns       = uniquePrefix("app-1")
		app2Ns       = uniquePrefix("app-2")
		istioMetrics = []string{
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
		metrics = map[string]string{
			"connection_security_policy":     "mutual_tls",
			"destination_app":                "destination",
			"destination_canonical_revision": "",
			"destination_canonical_service":  "",
			"destination_cluster":            "",
			"destination_principal":          fmt.Sprintf("spiffe://cluster.local/ns/%s/sa/default", app1Ns),
			"destination_service_name":       "destination",
			"destination_service_namespace":  app1Ns,
			"destination_service":            fmt.Sprintf("destination.%s.svc.cluster.local", app1Ns),
			"destination_version":            "",
			"destination_workload_namespace": "",
			"destination_workload":           "destination",
			"grpc_response_status":           "",
			"request_protocol":               "http",
			"response_code":                  "200",
			"response_flags":                 "",
			"source_app":                     "",
			"source_canonical_revision":      "",
			"source_canonical_service":       "",
			"source_cluster":                 "",
			"source_principal":               fmt.Sprintf("spiffe://cluster.local/ns/%s/sa/default", app1Ns),
			"source_version":                 "",
			"source_workload_namespace":      app1Ns,
			"source_workload":                "source",
		}
	)

	metricsKeys := make([]string, 0, len(metrics))
	for k := range metrics {
		metricsKeys = append(metricsKeys, k)
	}

	metricBackend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName("metrics"))
	logBackend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithName("logs"))

	metricPipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		WithOTLPInput(false).
		WithIstioInput(true, testutils.IncludeNamespaces(app1Ns)).
		WithOTLPOutput(testutils.OTLPEndpoint(metricBackend.EndpointHTTP())).
		Build()

	logPipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithRuntimeInput(false).
		WithOTLPOutput(testutils.OTLPEndpoint(logBackend.EndpointHTTP())).
		Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(genNs).K8sObject(),
		kitk8sobjects.NewNamespace(app1Ns, kitk8sobjects.WithIstioInjection()).K8sObject(),
		kitk8sobjects.NewNamespace(app2Ns, kitk8sobjects.WithIstioInjection()).K8sObject(),
		&metricPipeline,
		&logPipeline,
	}
	resources = append(resources, metricBackend.K8sObjects()...)
	resources = append(resources, logBackend.K8sObjects()...)
	resources = append(resources, trafficgen.K8sObjects(app1Ns)...)
	resources = append(resources, trafficgen.K8sObjects(app2Ns)...)
	resources = append(resources, telemetrygen.NewDeployment(genNs, telemetrygen.SignalTypeLogs).K8sObject())

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.DeploymentReady(t, kitkyma.MetricGatewayName)
	assert.DaemonSetReady(t, kitkyma.MetricAgentName)
	assert.BackendReachable(t, metricBackend)
	assert.BackendReachable(t, logBackend)

	assert.BackendDataEventuallyMatches(t, metricBackend, HaveFlatMetrics(
		ContainElement(HaveName(BeElementOf(istioMetrics))),
	))

	assert.BackendDataEventuallyMatches(t, metricBackend, HaveFlatMetrics(
		SatisfyAll(
			ContainElement(HaveResourceAttributes(SatisfyAll(
				HaveKeyWithValue("k8s.namespace.name", app1Ns),
				HaveKeyWithValue("k8s.pod.name", "destination"),
				HaveKeyWithValue("k8s.container.name", "istio-proxy"),
				HaveKeyWithValue("service.name", "destination"),
			))),
			ContainElement(HaveMetricAttributes(SatisfyAll(
				HaveKey(BeElementOf(metricsKeys)),
				HaveKeyWithValue("connection_security_policy", metrics["connection_security_policy"]),
				HaveKeyWithValue("destination_app", metrics["destination_app"]),
				HaveKeyWithValue("destination_principal", metrics["destination_principal"]),
				HaveKeyWithValue("destination_service_name", metrics["destination_service_name"]),
				HaveKeyWithValue("destination_service_namespace", metrics["destination_service_namespace"]),
				HaveKeyWithValue("destination_service", metrics["destination_service"]),
				HaveKeyWithValue("destination_workload", metrics["destination_workload"]),
				HaveKeyWithValue("request_protocol", metrics["request_protocol"]),
				HaveKeyWithValue("response_code", metrics["response_code"]),
				HaveKeyWithValue("source_principal", metrics["source_principal"]),
				HaveKeyWithValue("source_workload_namespace", metrics["source_workload_namespace"]),
				HaveKeyWithValue("source_workload", metrics["source_workload"]),
			))),
			ContainElement(HaveScopeName(ContainSubstring(common.InstrumentationScopeIstio))),
		),
	))

	assert.MetricsFromNamespaceDelivered(t, metricBackend, app1Ns, istioMetrics)
	assert.MetricsFromNamespaceNotDelivered(t, metricBackend, app2Ns)

	assert.BackendDataConsistentlyMatches(t, metricBackend, HaveFlatMetrics(
		Not(ContainElement(HaveMetricAttributes(HaveKeyWithValue("destination_workload", "telemetry-log-gateway")))),
	))

	assert.BackendDataConsistentlyMatches(t, metricBackend, HaveFlatMetrics(
		Not(ContainElement(HaveName(BeElementOf([]string{"up", "scrape_duration_seconds", "scrape_samples_scraped", "scrape_samples_post_metric_relabeling", "scrape_series_added"})))),
	))
}
