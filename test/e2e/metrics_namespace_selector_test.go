//go:build e2e

package e2e

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitmetric "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/metric"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/metricproducer"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Metrics Namespace Selector", Label("new"), func() {
	const (
		backendNs    = "metric-namespace-selector"
		backend1Name = "backend-1"
		backend2Name = "backend-2"
		app1Ns       = "namespace1"
		app2Ns       = "namespace2"
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

		metricPipeline1 := kitmetric.NewPipeline(backend1Name).
			WithOutputEndpointFromSecret(backend1.HostSecretRef()).
			PrometheusInput(true, &telemetryv1alpha1.MetricPipelineInputNamespaceSelector{
				Include: []string{app1Ns},
			}).
			RuntimeInput(true, &telemetryv1alpha1.MetricPipelineInputNamespaceSelector{
				Include: []string{app1Ns},
			}).
			OtlpInput(true, &telemetryv1alpha1.MetricPipelineInputNamespaceSelector{
				Include: []string{app1Ns},
			})
		objs = append(objs, metricPipeline1.K8sObject())

		backend2 := backend.New(backend2Name, backendNs, backend.SignalTypeMetrics)
		telemetryExportURLs[backend2Name] = backend2.TelemetryExportURL(proxyClient)
		objs = append(objs, backend2.K8sObjects()...)

		metricPipeline2 := kitmetric.NewPipeline(backend2Name).
			WithOutputEndpointFromSecret(backend2.HostSecretRef()).
			PrometheusInput(true, &telemetryv1alpha1.MetricPipelineInputNamespaceSelector{
				Exclude: []string{app1Ns},
			}).
			RuntimeInput(true, &telemetryv1alpha1.MetricPipelineInputNamespaceSelector{
				Exclude: []string{app1Ns},
			}).
			OtlpInput(true, &telemetryv1alpha1.MetricPipelineInputNamespaceSelector{
				Exclude: []string{app1Ns},
			})
		objs = append(objs, metricPipeline2.K8sObject())

		objs = append(objs,
			kitk8s.NewPod("app-1", app1Ns).WithPodSpec(telemetrygen.PodSpec(telemetrygen.SignalTypeMetrics)).K8sObject(),
			kitk8s.NewPod("app-2", app2Ns).WithPodSpec(telemetrygen.PodSpec(telemetrygen.SignalTypeMetrics)).K8sObject(),
			metricproducer.New(app1Ns).Pod().K8sObject(),
			metricproducer.New(app2Ns).Pod().K8sObject(),
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

		It("Should contain metrics from app1Ns in backend1", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURLs[backend1Name])
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ContainMd(
						ContainResourceAttrs(HaveKeyWithValue("k8s.namespace.name", app1Ns)),
					),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should contain no metrics from app2Ns in backend1", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURLs[backend1Name])
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
