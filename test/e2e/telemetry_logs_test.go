//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"time"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	kittraces "github.com/kyma-project/telemetry-manager/test/testkit/otel/traces"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Telemetry Components Error/Warning Logs", Label("telemetry-components"), Ordered, func() {
	const (
		mockNs             = "tlogs-http"
		logOTLPBackendName = "tlogs-log-otlp"
		metricBackendName  = "tlogs-metric"
		traceBackendName   = "tlogs-trace"
		nginxImage         = "europe-docker.pkg.dev/kyma-project/prod/external/nginx:1.23.3"
		curlImage          = "europe-docker.pkg.dev/kyma-project/prod/external/curlimages/curl:7.78.0"
		pushMetricsDepName = "push-metrics-istiofied"
	)

	var (
		logOTLPPipelineName       string
		metricPipelineName        string
		tracePipelineName         string
		logOTLPTelemetryExportURL string
		metricTelemetryExportURL  string
		traceTelemetryExportURL   string
		gomegaMaxLength           = format.MaxLength
		errorWarningLevels        = []string{
			"ERROR", "error",
			"WARNING", "warning",
			"WARN", "warn"}
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
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		// backends
		logOTLPBackend := backend.New(logOTLPBackendName, mockNs, backend.SignalTypeLogs)
		objs = append(objs, logOTLPBackend.K8sObjects()...)
		logOTLPTelemetryExportURL = logOTLPBackend.TelemetryExportURL(proxyClient)
		metricBackend := backend.New(metricBackendName, mockNs, backend.SignalTypeMetrics)
		metricTelemetryExportURL = metricBackend.TelemetryExportURL(proxyClient)
		objs = append(objs, metricBackend.K8sObjects()...)
		traceBackend := backend.New(traceBackendName, mockNs, backend.SignalTypeTraces)
		traceTelemetryExportURL = traceBackend.TelemetryExportURL(proxyClient)
		objs = append(objs, traceBackend.K8sObjects()...)

		// log pipelines
		logOTLPPipeline := kitk8s.NewLogPipelineV1Alpha1(fmt.Sprintf("%s-pipeline", logOTLPBackend.Name())).
			WithSecretKeyRef(logOTLPBackend.HostSecretRefV1Alpha1()).
			WithHTTPOutput().
			WithIncludeNamespaces([]string{kitkyma.SystemNamespaceName}).
			WithIncludeContainers([]string{"collector"})
		logOTLPPipelineName = logOTLPPipeline.Name()
		objs = append(objs, logOTLPPipeline.K8sObject())
		// TODO: Separate FluentBit logPipeline (CONTAINERS: fluent-bit, exporter)

		// metrics & traces
		metricPipeline := kitk8s.NewMetricPipelineV1Alpha1(fmt.Sprintf("%s-pipeline", metricBackend.Name())).
			WithOutputEndpointFromSecret(metricBackend.HostSecretRefV1Alpha1()).
			PrometheusInput(true, kitk8s.IncludeNamespacesV1Alpha1(mockNs)).
			IstioInput(true, kitk8s.IncludeNamespacesV1Alpha1(mockNs)).
			OtlpInput(true).
			RuntimeInput(true, kitk8s.IncludeNamespacesV1Alpha1(mockNs))
		metricPipelineName = metricPipeline.Name()
		objs = append(objs, metricPipeline.K8sObject())
		tracePipeline := kitk8s.NewTracePipelineV1Alpha1(fmt.Sprintf("%s-pipeline", traceBackend.Name())).
			WithOutputEndpointFromSecret(traceBackend.HostSecretRefV1Alpha1())
		tracePipelineName = tracePipeline.Name()
		objs = append(objs, tracePipeline.K8sObject())

		// metrics istio set-up (src/dest pods)
		source := kitk8s.NewPod("source", mockNs).WithPodSpec(sourcePodSpec())
		destination := kitk8s.NewPod("destination", mockNs).WithPodSpec(destinationPodSpec()).WithLabel("app", "destination")
		service := kitk8s.NewService("destination", mockNs).WithPort("http", 80)
		objs = append(objs, source.K8sObject(), destination.K8sObject(), service.K8sObject(kitk8s.WithLabel("app", "destination")))

		// metric istio set-up (telemetrygen)
		podSpec := telemetrygen.PodSpec(telemetrygen.SignalTypeMetrics, "")
		objs = append(objs,
			kitk8s.NewDeployment(pushMetricsDepName, mockNs).WithPodSpec(podSpec).K8sObject(),
		)

		return objs
	}

	Context("When telemetry components are set-up", func() {
		BeforeAll(func() {
			format.MaxLength = 0 // remove Gomega truncation
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have running metric and trace gateways", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.MetricGatewayName)
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.TraceGatewayName)
		})

		It("Should have running backends", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: logOTLPBackendName})
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: metricBackendName})
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: traceBackendName})
		})

		It("Should have running pipelines", func() {
			verifiers.LogPipelineShouldBeHealthy(ctx, k8sClient, logOTLPPipelineName)
			verifiers.MetricPipelineShouldBeHealthy(ctx, k8sClient, metricPipelineName)
			verifiers.TracePipelineShouldBeHealthy(ctx, k8sClient, tracePipelineName)
		})

		It("Should have a running metric agent daemonset", func() {
			verifiers.DaemonSetShouldBeReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		It("Should push metrics successfully", func() {
			verifiers.MetricsFromNamespaceShouldBeDelivered(proxyClient, metricTelemetryExportURL, mockNs, telemetrygen.MetricNames)
		})

		It("Should verify end-to-end trace delivery", func() {
			gatewayPushURL := proxyClient.ProxyURLForService(kitkyma.SystemNamespaceName, "telemetry-otlp-traces", "v1/traces/", ports.OTLPHTTP)
			traceID, spanIDs, attrs := kittraces.MakeAndSendTraces(proxyClient, gatewayPushURL)
			verifiers.TracesShouldBeDelivered(proxyClient, traceTelemetryExportURL, traceID, spanIDs, attrs)
		})

		// whitelist possible (flaky/expected) errors
		excludeWhitelistedLogs := func() gomegatypes.GomegaMatcher {
			return Or(
				ContainSubstring("The default endpoints for all servers in components will change to use localhost instead of 0.0.0.0 in a future version. Use the feature gate to preview the new default."),
				ContainSubstring("error re-reading certificate"),
				ContainSubstring("grpc: addrConn.createTransport failed to connect"),
			)
		}

		It("Should not have any ERROR/WARNING logs in the OTLP containers", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(logOTLPTelemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					Not(ContainLd(ContainLogRecord(SatisfyAll(
						WithPodName(ContainSubstring("telemetry-")),
						WithLevel(BeElementOf(errorWarningLevels)),
						WithLogBody(Not(excludeWhitelistedLogs())),
					)))),
				))
			}, time.Second*120, periodic.TelemetryInterval).Should(Succeed())
		})

		// TODO: Should not have any ERROR/WARNING level logs in the FluentBit containers
		// TODO: configmap: FLuentBit, exclude_path (excluding self logs)
		// telemetry-manager/blob/test/check-error-logs/internal/fluentbit/config/builder/input.go#L15-L16

		AfterAll(func() {
			format.MaxLength = gomegaMaxLength // restore Gomega truncation
		})
	})
})
