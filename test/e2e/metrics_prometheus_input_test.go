//go:build e2e

package e2e

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitmetric "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/metric"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/metricproducer"
	"github.com/kyma-project/telemetry-manager/test/testkit/otlp/kubeletstats"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Metrics Prometheus Input", Label("metrics"), func() {
	const (
		backendNs   = "metric-prometheus-input"
		backendName = "metric-agent-receiver"
		app1Ns      = "namespace1"
		app2Ns      = "namespace2"
	)

	var (
		pipelineName       string
		telemetryExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(backendNs).K8sObject(),
			kitk8s.NewNamespace(app1Ns).K8sObject(),
			kitk8s.NewNamespace(app2Ns).K8sObject())

		mockBackend := backend.New(backendName, backendNs, backend.SignalTypeMetrics)
		telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)
		objs = append(objs, mockBackend.K8sObjects()...)

		app1 := metricproducer.New(app1Ns)
		objs = append(objs, []client.Object{
			app1.Pod().WithPrometheusAnnotations(metricproducer.SchemeHTTP).K8sObject(),
			app1.Service().WithPrometheusAnnotations(metricproducer.SchemeHTTP).K8sObject(),
		}...)

		app2 := metricproducer.New(app2Ns)
		objs = append(objs, []client.Object{
			app2.Pod().WithPrometheusAnnotations(metricproducer.SchemeHTTP).K8sObject(),
			app2.Service().WithPrometheusAnnotations(metricproducer.SchemeHTTP).K8sObject(),
		}...)

		metricPipeline := kitmetric.NewPipeline("pipeline-with-prometheus-input-enabled").
			WithOutputEndpointFromSecret(mockBackend.HostSecretRef()).
			PrometheusInput(true, &telemetryv1alpha1.MetricPipelineInputNamespaceSelector{
				Include: []string{app1Ns},
			})
		pipelineName = metricPipeline.Name()
		objs = append(objs, metricPipeline.K8sObject())

		return objs
	}

	Context("When a metricpipeline with prometheus input exists", Ordered, func() {
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

		It("Should have a running pipeline", func() {
			verifiers.MetricPipelineShouldBeRunning(ctx, k8sClient, pipelineName)
		})

		It("Should verify custom metric scraping via annotated pods", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				// here we are discovering the same metric-producer workload twice: once via the annotated service and once via the annotated pod
				// targets discovered via annotated pods must have no service label
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainMd(ContainMetric(SatisfyAll(
						WithName(Equal(metricproducer.MetricCPUTemperature.Name)),
						WithType(Equal(metricproducer.MetricCPUTemperature.Type)),
					))),
					ContainMd(ContainMetric(SatisfyAll(
						WithName(Equal(metricproducer.MetricCPUEnergyHistogram.Name)),
						WithType(Equal(metricproducer.MetricCPUEnergyHistogram.Type)),
					))),
					ContainMd(ContainMetric(SatisfyAll(
						WithName(Equal(metricproducer.MetricHardwareHumidity.Name)),
						WithType(Equal(metricproducer.MetricHardwareHumidity.Type)),
					))),
					ContainMd(ContainMetric(SatisfyAll(
						WithName(Equal(metricproducer.MetricHardDiskErrorsTotal.Name)),
						WithType(Equal(metricproducer.MetricHardDiskErrorsTotal.Type)),
					))),
				),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should verify custom metric scraping via annotated services", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainMd(ContainMetric(SatisfyAll(
						WithName(Equal(metricproducer.MetricCPUTemperature.Name)),
						WithType(Equal(metricproducer.MetricCPUTemperature.Type)),
						ContainDataPointAttrs(HaveKey("service")),
					))),
					ContainMd(ContainMetric(SatisfyAll(
						WithName(Equal(metricproducer.MetricCPUEnergyHistogram.Name)),
						WithType(Equal(metricproducer.MetricCPUEnergyHistogram.Type)),
						ContainDataPointAttrs(HaveKey("service")),
					))),
					ContainMd(ContainMetric(SatisfyAll(
						WithName(Equal(metricproducer.MetricHardwareHumidity.Name)),
						WithType(Equal(metricproducer.MetricHardwareHumidity.Type)),
						ContainDataPointAttrs(HaveKey("service")),
					))),
					ContainMd(ContainMetric(SatisfyAll(
						WithName(Equal(metricproducer.MetricHardDiskErrorsTotal.Name)),
						WithType(Equal(metricproducer.MetricHardDiskErrorsTotal.Type)),
						ContainDataPointAttrs(HaveKey("service")),
					))),
				),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should verify no kubelet metrics", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					Not(ContainMd(ContainMetric(WithName(BeElementOf(kubeletstats.MetricNames))))),
				))
			}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should contain metrics with service.name set to telemetry-metric-gateway", Label(operationalTest), func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ContainMd(
						ContainResourceAttrs(HaveKeyWithValue("service.name", kitkyma.MetricGatewayBaseName)),
					),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should contain metrics with service.name set to telemetry-metric-agent", Label(operationalTest), func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ContainMd(
						ContainResourceAttrs(HaveKeyWithValue("service.name", kitkyma.MetricAgentBaseName)),
					),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should contain metrics from app1Ns", Label(operationalTest), func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ContainMd(
						ContainResourceAttrs(HaveKeyWithValue("k8s.namespace.name", app1Ns)),
					),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should contain no metrics from app2Ns", Label(operationalTest), func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					Not(ContainMd(
						ContainResourceAttrs(HaveKeyWithValue("k8s.namespace.name", app2Ns)),
					)),
				))
			}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
