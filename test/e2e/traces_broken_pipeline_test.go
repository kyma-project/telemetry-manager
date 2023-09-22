//go:build e2e

package e2e

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kittrace "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/trace"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"
	kittraces "github.com/kyma-project/telemetry-manager/test/testkit/otlp/traces"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Traces", Label("tracing"), func() {
	const (
		mockBackendName    = "metric-receiver"
		mockNs             = "metric-mocks-broken-pipeline"
		brokenPipelineName = "broken-trace-pipeline"
	)
	var (
		pipelineName string
		urls         *urlprovider.URLProvider
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeTraces)
		objs = append(objs, mockBackend.K8sObjects()...)
		urls.SetMockBackendExport(mockBackend.Name(), mockBackend.TelemetryExportURL(proxyClient))

		pipeline := kittrace.NewPipeline(fmt.Sprintf("%s-%s", mockBackend.Name(), "pipeline"), mockBackend.HostSecretRefKey())
		pipelineName = pipeline.Name()
		objs = append(objs, pipeline.K8sObject())

		hostSecret := kitk8s.NewOpaqueSecret("metric-rcv-hostname-"+brokenPipelineName, kitkyma.DefaultNamespaceName, kitk8s.WithStringData("metric-host", "http://unreachable:4317"))
		brokenTracePipeline := kittrace.NewPipeline(brokenPipelineName, hostSecret.SecretKeyRef("metric-host"))
		brokenPipelineObjs := []client.Object{hostSecret.K8sObject(), brokenTracePipeline.K8sObject()}
		objs = append(objs, brokenPipelineObjs...)

		urls.SetOTLPPush(proxyClient.ProxyURLForService(
			kitkyma.DefaultNamespaceName, "telemetry-otlp-traces", "v1/traces/", ports.OTLPHTTP),
		)

		return objs
	}
	Context("When a broken tracepipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			verifiers.TracePipelineShouldBeRunning(ctx, k8sClient, pipelineName)
			verifiers.TracePipelineShouldBeRunning(ctx, k8sClient, brokenPipelineName)
		})

		It("Should have a running trace collector deployment", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.TraceGatewayName)

		})

		It("Should have a trace backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName, Namespace: mockNs})
		})

		It("Should verify end-to-end trace delivery for the remaining pipeline", func() {
			traceID, spanIDs, attrs := kittraces.MakeAndSendTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldBeDelivered(proxyClient, urls.MockBackendExport(mockBackendName), traceID, spanIDs, attrs)
		})
	})

})
