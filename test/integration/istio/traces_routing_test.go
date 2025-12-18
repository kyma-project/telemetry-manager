package istio

import (
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/trace"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

// TestTracesRouting verifies trace delivery and Istio integration in a scenario with two trace backends and two trace generators (apps), each paired with a dedicated trace pipeline.
// One backend and one app are deployed with Istio sidecar injection enabled (inside the mesh), while the other backend and app are deployed without Istio (outside the mesh).
// The test validates that traces are correctly routed to the appropriate backends, Istio sidecar injection is functioning as expected, and only the desired spans are present in the collected traces.
func TestTracesRouting(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelGardener, suite.LabelIstio)

	const (
		appName          = "app"
		istiofiedAppName = "app-istiofied"
		istiofiedAppNs   = permissiveNs
	)

	var (
		uniquePrefix       = unique.Prefix()
		pipeline1Name      = uniquePrefix("1")
		pipeline2Name      = uniquePrefix("2")
		backendNs          = uniquePrefix("backend")
		istiofiedBackendNs = uniquePrefix("backend-istiofied")
		appNs              = uniquePrefix("app")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeTraces)
	istiofiedBackend := kitbackend.New(istiofiedBackendNs, kitbackend.SignalTypeTraces)

	tracePipeline := testutils.NewTracePipelineBuilder().
		WithName(pipeline1Name).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
		Build()

	istioTracePipeline := testutils.NewTracePipelineBuilder().
		WithName(pipeline2Name).
		WithOTLPOutput(testutils.OTLPEndpoint(istiofiedBackend.EndpointHTTP())).
		Build()

	traceGatewayExternalService := kitk8sobjects.NewService("telemetry-otlp-traces-external", kitkyma.SystemNamespaceName).
		WithPort("grpc-otlp", ports.OTLPGRPC).
		WithPort("http-metrics", ports.Metrics)
	metricServiceURL := suite.ProxyClient.ProxyURLForService(kitkyma.SystemNamespaceName, "telemetry-otlp-traces-external", "metrics", ports.Metrics)

	app := prommetricgen.New(appNs, prommetricgen.WithName(appName))
	appURL := app.PodURL(suite.ProxyClient)

	istiofiedApp := prommetricgen.New(istiofiedAppNs, prommetricgen.WithName(istiofiedAppName))
	istiofiedAppURL := istiofiedApp.PodURL(suite.ProxyClient)

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(istiofiedBackendNs, kitk8sobjects.WithIstioInjection()).K8sObject(),
		kitk8sobjects.NewNamespace(appNs).K8sObject(),
		&istioTracePipeline,
		&tracePipeline,
		traceGatewayExternalService.K8sObject(kitk8sobjects.WithLabel("app.kubernetes.io/name", "telemetry-trace-gateway")),
		app.Pod().K8sObject(),
		istiofiedApp.Pod().K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)
	resources = append(resources, istiofiedBackend.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.BackendReachable(t, istiofiedBackend)

	assertAppIsRunning(t, istiofiedAppNs, map[string]string{"app.kubernetes.io/name": "metric-producer"})
	assertSidecarPresent(t, istiofiedAppNs, map[string]string{"app.kubernetes.io/name": "metric-producer"})
	assertAppIsRunning(t, appNs, map[string]string{"app.kubernetes.io/name": "metric-producer"})
	assert.DeploymentReady(t, kitkyma.TraceGatewayName)
	assert.TracePipelineHealthy(t, pipeline1Name)
	assert.TracePipelineHealthy(t, pipeline2Name)

	Eventually(func(g Gomega) {
		resp, err := suite.ProxyClient.Get(metricServiceURL)
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		defer resp.Body.Close()
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())

	for _, podURLs := range []string{appURL, istiofiedAppURL} {
		for range 100 {
			Eventually(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(podURLs)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				defer resp.Body.Close()
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		}
	}

	assertIstioSpans(t, backend, istiofiedAppNs)
	assertIstioSpans(t, istiofiedBackend, istiofiedAppNs)
	assertCustomAppSpans(t, backend, istiofiedAppName, istiofiedAppNs)
	assertCustomAppSpans(t, istiofiedBackend, istiofiedAppName, istiofiedAppNs)
	assertCustomAppSpans(t, backend, appName, appNs)
	assertCustomAppSpans(t, istiofiedBackend, appName, appNs)
	assertNoIstioNoiseSpans(t, backend)
	assertNoIstioNoiseSpans(t, istiofiedBackend)
}

func assertSidecarPresent(t *testing.T, namespace string, labelSelector map[string]string) {
	t.Helper()

	Eventually(func(g Gomega) {
		listOptions := client.ListOptions{
			LabelSelector: labels.SelectorFromSet(labelSelector),
			Namespace:     namespace,
		}

		hasIstioSidecar, err := assert.PodsHaveContainer(t, listOptions, "istio-proxy")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(hasIstioSidecar).To(BeTrueBecause("Istio sidecar not present"))
	}, periodic.EventuallyTimeout*2, periodic.DefaultInterval).Should(Succeed())
}

func assertAppIsRunning(t *testing.T, namespace string, labelSelector map[string]string) {
	t.Helper()

	listOptions := client.ListOptions{
		LabelSelector: labels.SelectorFromSet(labelSelector),
		Namespace:     namespace,
	}

	assert.PodsReady(t, listOptions)
}

func assertIstioSpans(t *testing.T, backend *kitbackend.Backend, namespace string) {
	t.Helper()

	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatTraces(ContainElement(SatisfyAll(
			// Identify istio-proxy traces by component=proxy attribute
			HaveSpanAttributes(HaveKeyWithValue("component", "proxy")),
			HaveSpanAttributes(HaveKeyWithValue("istio.namespace", namespace)),
		))),
	)
}

func assertNoIstioNoiseSpans(t *testing.T, backend *kitbackend.Backend) {
	t.Helper()

	assert.BackendDataEventuallyMatches(t, backend,
		Not(HaveFlatTraces(ContainElement(SatisfyAll(
			// Identify istio-proxy traces by component=proxy attribute
			HaveSpanAttributes(HaveKeyWithValue("component", "proxy")),
			// All calls to telemetry-otlp-traces should be dropped
			HaveSpanAttributes(HaveKeyWithValue("http.url", "http://telemetry-otlp-traces.kyma-system.svc.cluster.local:4318/v1/traces")),
		)))),
	)
}

func assertCustomAppSpans(t *testing.T, backend *kitbackend.Backend, name, namespace string) {
	t.Helper()

	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatTraces(ContainElement(SatisfyAll(
			// Identify sample app by serviceName attribute
			HaveResourceAttributes(HaveKeyWithValue("service.name", "metric-producer")),
			HaveResourceAttributes(HaveKeyWithValue("k8s.pod.name", name)),
			HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", namespace)),
		))),
	)
}
