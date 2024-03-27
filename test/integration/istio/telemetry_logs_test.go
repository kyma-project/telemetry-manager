//go:build istio

package istio

import (
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

// TODO: Change to logs label
var _ = Describe("Telemetry Components Error/Warning Logs", Label("wip"), Ordered, func() {
	const (
		mockNs            = "tlogs-http"
		logBackendName    = "tlogs-log"
		metricBackendName = "tlogs-metric"
		traceBackendName  = "tlogs-trace"
		nginxImage        = "europe-docker.pkg.dev/kyma-project/prod/external/nginx:1.23.3"
		curlImage         = "europe-docker.pkg.dev/kyma-project/prod/external/curlimages/curl:7.78.0"
	)

	var (
		logPipelineName       string
		metricPipelineName    string
		tracePipelineName     string
		logTelemetryExportURL string
		now                   time.Time
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
		logBackend := backend.New(logBackendName, mockNs, backend.SignalTypeLogs)
		objs = append(objs, logBackend.K8sObjects()...)
		logTelemetryExportURL = logBackend.TelemetryExportURL(proxyClient)
		metricBackend := backend.New(metricBackendName, mockNs, backend.SignalTypeMetrics)
		objs = append(objs, metricBackend.K8sObjects()...)
		traceBackend := backend.New(traceBackendName, mockNs, backend.SignalTypeTraces)
		objs = append(objs, traceBackend.K8sObjects()...)
		// TODO: Generate some traces (see other traces test-cases)

		// metric istio set-up
		source := kitk8s.NewPod("source", mockNs).WithPodSpec(sourcePodSpec())
		destination := kitk8s.NewPod("destination", mockNs).WithPodSpec(destinationPodSpec()).WithLabel("app", "destination")
		service := kitk8s.NewService("destination", mockNs).WithPort("http", 80)
		objs = append(objs, source.K8sObject(), destination.K8sObject(), service.K8sObject(kitk8s.WithLabel("app", "destination")))

		// components
		logPipeline := kitk8s.NewLogPipelineV1Alpha1(fmt.Sprintf("%s-pipeline", logBackend.Name())).
			WithSecretKeyRef(logBackend.HostSecretRefV1Alpha1()).
			WithHTTPOutput().
			WithIncludeNamespaces([]string{kitkyma.SystemNamespaceName})
		logPipelineName = logPipeline.Name()
		objs = append(objs, logPipeline.K8sObject())
		metricPipeline := kitk8s.NewMetricPipelineV1Alpha1(fmt.Sprintf("%s-pipeline", metricBackend.Name())).
			WithOutputEndpointFromSecret(metricBackend.HostSecretRefV1Alpha1()).
			PrometheusInput(true, kitk8s.IncludeNamespacesV1Alpha1(mockNs)).
			IstioInput(true, kitk8s.IncludeNamespacesV1Alpha1(mockNs)).
			RuntimeInput(true, kitk8s.IncludeNamespacesV1Alpha1(mockNs))
		metricPipelineName = metricPipeline.Name()
		objs = append(objs, metricPipeline.K8sObject())
		tracePipeline := kitk8s.NewTracePipelineV1Alpha1(fmt.Sprintf("%s-pipeline", traceBackend.Name())).
			WithOutputEndpointFromSecret(traceBackend.HostSecretRefV1Alpha1())
		tracePipelineName = tracePipeline.Name()
		objs = append(objs, tracePipeline.K8sObject())

		return objs
	}

	Context("When telemetry components are set-up", func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			// TEST: Comment-out for debugging
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
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: logBackendName})
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: metricBackendName})
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: traceBackendName})
		})

		It("Should have running pipelines", func() {
			verifiers.LogPipelineShouldBeHealthy(ctx, k8sClient, logPipelineName)
			verifiers.MetricPipelineShouldBeHealthy(ctx, k8sClient, metricPipelineName)
			verifiers.TracePipelineShouldBeHealthy(ctx, k8sClient, tracePipelineName)
		})

		It("Should have a running metric agent daemonset", func() {
			verifiers.DaemonSetShouldBeReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		// TODO: Whitelist possible (flaky/expected) errors
		// excludeWhitelistedLogs := func() string {
		// }

		It("Should not have any ERROR/WARNING level logs in the components", func() {
			now = time.Now().UTC()
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(logTelemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					Not(ContainLd(ContainLogRecord(SatisfyAll(
						WithPodName(ContainSubstring("telemetry-")),
						WithLevel(Or(Equal("ERROR"), Equal("WARNING"))),
						WithTimestamp(BeTemporally(">=", now)),
					)))),
				))
			}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})

	// configmap: FLuentBit, exclude_path (excluding self logs)
	// https://vscode.dev/github/TeodorSAP/telemetry-manager/blob/test/check-error-logs/internal/fluentbit/config/builder/input.go#L15-L16
})
