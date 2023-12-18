//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitmetricpipeline "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/metricproducer"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/otlp/kubeletstats"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Metrics Namespace Selector", Label("metrics"), func() {
	const (
		backendNs    = "metric-namespace-selector"
		backend1Name = "backend-1"
		backend2Name = "backend-2"

		app1Ns = "app-1"
		app2Ns = "app-2"
	)
	var (
		telemetryExportURLs = make(map[string]string)
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(backendNs).K8sObject(),
			kitk8s.NewNamespace(app1Ns).K8sObject(),
			kitk8s.NewNamespace(app2Ns).K8sObject())

		backend1 := backend.New(backend1Name, backendNs, backend.SignalTypeMetrics)
		telemetryExportURLs[backend1Name] = backend1.TelemetryExportURL(proxyClient)
		objs = append(objs, backend1.K8sObjects()...)

		pipelineIncludeApp1Ns := kitmetricpipeline.NewPipeline("include-"+app1Ns).
			WithOutputEndpointFromSecret(backend1.HostSecretRef()).
			PrometheusInput(true, kitmetricpipeline.IncludeNamespaces(app1Ns)).
			RuntimeInput(true, kitmetricpipeline.IncludeNamespaces(app1Ns)).
			OtlpInput(true, kitmetricpipeline.IncludeNamespaces(app1Ns))
		objs = append(objs, pipelineIncludeApp1Ns.K8sObject())

		backend2 := backend.New(backend2Name, backendNs, backend.SignalTypeMetrics)
		telemetryExportURLs[backend2Name] = backend2.TelemetryExportURL(proxyClient)
		objs = append(objs, backend2.K8sObjects()...)

		pipelineExcludeApp1Ns := kitmetricpipeline.NewPipeline("exclude-"+app1Ns).
			WithOutputEndpointFromSecret(backend2.HostSecretRef()).
			PrometheusInput(true, kitmetricpipeline.ExcludeNamespaces(app1Ns)).
			RuntimeInput(true, kitmetricpipeline.ExcludeNamespaces(app1Ns)).
			OtlpInput(true, kitmetricpipeline.ExcludeNamespaces(app1Ns))
		objs = append(objs, pipelineExcludeApp1Ns.K8sObject())

		objs = append(objs,
			telemetrygen.New(app1Ns).K8sObject(),
			telemetrygen.New(app2Ns).K8sObject(),
			telemetrygen.New(kitkyma.SystemNamespaceName).K8sObject(),

			metricproducer.New(app1Ns).Pod().WithPrometheusAnnotations(metricproducer.SchemeHTTP).K8sObject(),
			metricproducer.New(app2Ns).Pod().WithPrometheusAnnotations(metricproducer.SchemeHTTP).K8sObject(),
			metricproducer.New(kitkyma.SystemNamespaceName).Pod().WithPrometheusAnnotations(metricproducer.SchemeHTTP).K8sObject(),
		)

		return objs
	}

	Context("When multiple metricpipelines exist", Ordered, func() {
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
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: backend1Name, Namespace: backendNs})
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: backend2Name, Namespace: backendNs})
		})

		It("Should have a running metric agent daemonset", func() {
			verifiers.DaemonSetShouldBeReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		// verify metrics from apps1Ns delivered to backend1
		It("Should deliver Runtime metrics from app1Ns to backend1", func() {
			verifiers.MetricsFromNamespaceShouldBeDelivered(proxyClient, telemetryExportURLs[backend1Name], app1Ns, kubeletstats.MetricNames)
		})

		It("Should deliver Prometheus metrics from app1Ns to backend1", func() {
			verifiers.MetricsFromNamespaceShouldBeDelivered(proxyClient, telemetryExportURLs[backend1Name], app1Ns, metricproducer.MetricNames)
		})

		It("Should deliver OTLP metrics from app1Ns to backend1", func() {
			verifiers.MetricsFromNamespaceShouldBeDelivered(proxyClient, telemetryExportURLs[backend1Name], app1Ns, telemetrygen.MetricNames)
		})

		It("Should not deliver metrics from app2Ns to backend1", func() {
			verifiers.MetricsFromNamespaceShouldNotBeDelivered(proxyClient, telemetryExportURLs[backend1Name], app2Ns)
		})

		// verify metrics from apps2Ns delivered to backend1
		It("Should deliver Runtime metrics from app2Ns to backend2", func() {
			verifiers.MetricsFromNamespaceShouldBeDelivered(proxyClient, telemetryExportURLs[backend2Name], app2Ns, kubeletstats.MetricNames)
		})

		It("Should deliver Prometheus metrics from app2Ns to backend2", func() {
			verifiers.MetricsFromNamespaceShouldBeDelivered(proxyClient, telemetryExportURLs[backend2Name], app2Ns, metricproducer.MetricNames)
		})

		It("Should deliver OTLP metrics from app2Ns to backend2", func() {
			verifiers.MetricsFromNamespaceShouldBeDelivered(proxyClient, telemetryExportURLs[backend2Name], app2Ns, telemetrygen.MetricNames)
		})

		It("Should not deliver metrics from app1Ns to backend2", func() {
			verifiers.MetricsFromNamespaceShouldNotBeDelivered(proxyClient, telemetryExportURLs[backend2Name], app1Ns)
		})
	})
})
