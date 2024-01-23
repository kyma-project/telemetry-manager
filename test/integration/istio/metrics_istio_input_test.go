//go:build istio

package istio

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Metrics Istio Input", Label("metrics"), func() {
	const (
		backendNs   = "istio-metric-istio-input"
		backendName = "backend"
		app1Ns      = "app-1"
		app2Ns      = "app-2"
		nginxImage  = "europe-docker.pkg.dev/kyma-project/prod/external/nginx:1.23.3"
		curlImage   = "europe-docker.pkg.dev/kyma-project/prod/external/curlimages/curl:7.78.0"
	)

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
		telemetryExportURL string
	)

	sourcePodSpec := func() corev1.PodSpec {
		return corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "source",
					Image: curlImage,
					Command: []string{
						"/bin/sh",
						"-c",
						"while true; do curl http://destination:80; sleep 1; done",
					},
				},
			},
		}
	}

	destinationPodSpec := func() corev1.PodSpec {
		return corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "destination",
					Image: nginxImage,
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 80,
							Protocol:      corev1.ProtocolTCP,
						},
					},
				},
			},
		}
	}

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(backendNs).K8sObject(),
			kitk8s.NewNamespace(app1Ns, kitk8s.WithIstioInjection()).K8sObject(),
			kitk8s.NewNamespace(app2Ns, kitk8s.WithIstioInjection()).K8sObject())

		mockBackend := backend.New(backendName, backendNs, backend.SignalTypeMetrics)
		objs = append(objs, mockBackend.K8sObjects()...)
		telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)

		metricPipeline := kitk8s.NewMetricPipeline("pipeline-with-istio-input-enabled").
			WithOutputEndpointFromSecret(mockBackend.HostSecretRef()).
			OtlpInput(false).
			IstioInput(true, kitk8s.IncludeNamespaces(app1Ns))
		objs = append(objs, metricPipeline.K8sObject())

		source1 := kitk8s.NewPod("source", app1Ns).WithPodSpec(sourcePodSpec())
		destination1 := kitk8s.NewPod("destination", app1Ns).WithPodSpec(destinationPodSpec()).WithLabel("app", "destination")
		service1 := kitk8s.NewService("destination", app1Ns).WithPort("http", 80)

		source2 := kitk8s.NewPod("source", app2Ns).WithPodSpec(sourcePodSpec())
		destination2 := kitk8s.NewPod("destination", app2Ns).WithPodSpec(destinationPodSpec()).WithLabel("app", "destination")
		service2 := kitk8s.NewService("destination", app2Ns).WithPort("http", 80)

		objs = append(objs, source1.K8sObject(), destination1.K8sObject(), service1.K8sObject(kitk8s.WithLabel("app", "destination")), source2.K8sObject(), destination2.K8sObject(), service2.K8sObject(kitk8s.WithLabel("app", "destination")))

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
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Should have a metrics backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: backendName, Namespace: backendNs})
		})

		It("Should have a running metric agent daemonset", func() {
			verifiers.DaemonSetShouldBeReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		It("Should verify istio proxy metric scraping", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ContainMd(ContainMetric(WithName(BeElementOf(istioProxyMetricNames)))),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should verify istio proxy metric attributes", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ContainMd(SatisfyAll(
						ContainResourceAttrs(SatisfyAll(
							HaveKeyWithValue("k8s.namespace.name", "app-1"),
							HaveKeyWithValue("k8s.pod.name", "destination"),
							HaveKeyWithValue("k8s.container.name", "istio-proxy"),
							HaveKeyWithValue("service.name", "destination"),
						)),
						ContainMetric(SatisfyAll(
							ContainDataPointAttrs(HaveKey(BeElementOf(istioProxyMetricAttributes))),
							ContainDataPointAttrs(HaveKeyWithValue("source_workload_namespace", app1Ns)),
							ContainDataPointAttrs(HaveKeyWithValue("destination_workload", "destination")),
							ContainDataPointAttrs(HaveKeyWithValue("destination_app", "destination")),
							ContainDataPointAttrs(HaveKeyWithValue("destination_service_name", "destination")),
							ContainDataPointAttrs(HaveKeyWithValue("destination_service", "destination.app-1.svc.cluster.local")),
							ContainDataPointAttrs(HaveKeyWithValue("destination_service_namespace", app1Ns)),
							ContainDataPointAttrs(HaveKeyWithValue("destination_principal", "spiffe://cluster.local/ns/app-1/sa/default")),
							ContainDataPointAttrs(HaveKeyWithValue("source_workload", "source")),
							ContainDataPointAttrs(HaveKeyWithValue("source_principal", "spiffe://cluster.local/ns/app-1/sa/default")),
							ContainDataPointAttrs(HaveKeyWithValue("response_code", "200")),
							ContainDataPointAttrs(HaveKeyWithValue("request_protocol", "http")),
							ContainDataPointAttrs(HaveKeyWithValue("connection_security_policy", "mutual_tls")),
						)))),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should deliver metrics from app-1 namespace", func() {
			verifiers.MetricsFromNamespaceShouldBeDelivered(proxyClient, telemetryExportURL, app1Ns, istioProxyMetricNames)
		})

		It("Should not deliver metrics from app-2 namespace", func() {
			verifiers.MetricsFromNamespaceShouldNotBeDelivered(proxyClient, telemetryExportURL, app2Ns)
		})

		It("Should verify that istio metric with source_workload=telemetry-metric-agent does not exist", func() {
			verifyMetricIsNotPresent(telemetryExportURL, "source_workload", "telemetry-telemetry-gateway")
		})
		It("Should verify that istio metric with destination_workload=telemetry-metric-gateway does not exist", func() {
			verifyMetricIsNotPresent(telemetryExportURL, "destination_workload", "telemetry-metric-gateway")
		})

		It("Ensures no diagnostic metrics are sent to backend", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					Not(ContainMd(ContainMetric(WithName(BeElementOf("up", "scrape_duration_seconds", "scrape_samples_scraped", "scrape_samples_post_metric_relabeling", "scrape_series_added"))))),
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
		g.Expect(resp).NotTo(HaveHTTPBody(
			ContainMd(ContainMetric(SatisfyAll(
				ContainDataPointAttrs(HaveKeyWithValue(key, value)),
			))),
		))
	}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
}
