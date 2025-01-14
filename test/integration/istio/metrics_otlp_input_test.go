//go:build istio

package istio

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelIntegration), Ordered, func() {
	const (
		metricProducer1Name = "metric-producer-1"
		metricProducer2Name = "metric-producer-2"
	)

	var (
		backendNs          = suite.ID()
		istiofiedBackendNs = suite.IDWithSuffix("istiofied")

		pipeline1Name             = suite.IDWithSuffix("1")
		pipeline2Name             = suite.IDWithSuffix("2")
		backendExportURL          string
		istiofiedBackendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(backendNs).K8sObject())
		objs = append(objs, kitk8s.NewNamespace(istiofiedBackendNs, kitk8s.WithIstioInjection()).K8sObject())

		// Mocks namespace objects
		backend1 := backend.New(backendNs, backend.SignalTypeMetrics)
		objs = append(objs, backend1.K8sObjects()...)
		backendExportURL = backend1.ExportURL(proxyClient)

		backend2 := backend.New(istiofiedBackendNs, backend.SignalTypeMetrics)
		objs = append(objs, backend2.K8sObjects()...)
		istiofiedBackendExportURL = backend2.ExportURL(proxyClient)

		metricPipeline := testutils.NewMetricPipelineBuilder().
			WithName(pipeline1Name).
			WithOTLPOutput(testutils.OTLPEndpoint(backend1.Endpoint())).
			Build()
		objs = append(objs, &metricPipeline)

		metricPipelineIstiofiedBackend := testutils.NewMetricPipelineBuilder().
			WithName(pipeline2Name).
			WithOTLPOutput(testutils.OTLPEndpoint(backend2.Endpoint())).
			Build()

		objs = append(objs, &metricPipelineIstiofiedBackend)

		// set peerauthentication to strict explicitly
		peerAuth := kitk8s.NewPeerAuthentication(backend.DefaultName, istiofiedBackendNs)
		objs = append(objs, peerAuth.K8sObject(kitk8s.WithLabel("app", backend.DefaultName)))

		// Create 2 deployments (with and without side-car) which would push the metrics to the metrics gateway.
		podSpec := telemetrygen.PodSpec(telemetrygen.SignalTypeMetrics)
		objs = append(objs,
			kitk8s.NewDeployment(metricProducer1Name, backendNs).WithPodSpec(podSpec).K8sObject(),
			kitk8s.NewDeployment(metricProducer2Name, istiofiedBackendNs).WithPodSpec(podSpec).K8sObject(),
		)

		return objs
	}

	Context("Istiofied and non-istiofied in-cluster backends", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
				for _, resource := range k8sObjects {
					Eventually(func(g Gomega) {
						key := types.NamespacedName{Name: resource.GetName(), Namespace: resource.GetNamespace()}
						err := k8sClient.Get(ctx, key, resource)
						g.Expect(apierrors.IsNotFound(err)).To(BeTrueBecause("Resource %s still exists", key))
					}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
				}
			})

			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running metric gateway deployment", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Should have a metrics backend running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: backendNs})
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: istiofiedBackendNs})
		})

		It("Should push metrics successfully", func() {
			assert.MetricsFromNamespaceDelivered(proxyClient, backendExportURL, backendNs, telemetrygen.MetricNames)
			assert.MetricsFromNamespaceDelivered(proxyClient, backendExportURL, istiofiedBackendNs, telemetrygen.MetricNames)

			assert.MetricsFromNamespaceDelivered(proxyClient, istiofiedBackendExportURL, backendNs, telemetrygen.MetricNames)
			assert.MetricsFromNamespaceDelivered(proxyClient, istiofiedBackendExportURL, istiofiedBackendNs, telemetrygen.MetricNames)

		})
	})
})
