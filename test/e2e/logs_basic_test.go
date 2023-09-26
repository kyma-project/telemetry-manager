//go:build e2e

package e2e

import (
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitlog "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/logproducer"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type OutputType string

const (
	OutputTypeHTTP   = "http"
	OutputTypeCustom = "custom"
)

var _ = Describe("Logs Basic", Label("logging"), func() {
	var urls = urlprovider.New()

	makeResources := func(mockNs, mockBackendName, logProducerName string, outputType OutputType) []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeLogs)
		mockLogProducer := logproducer.New(logProducerName, mockNs)
		objs = append(objs, mockBackend.K8sObjects()...)
		objs = append(objs, mockLogProducer.K8sObject(kitk8s.WithLabel("app", "logging-test")))
		urls.SetMockBackendExport(mockBackend.Name(), mockBackend.TelemetryExportURL(proxyClient))

		var logPipeline *kitlog.Pipeline
		if outputType == OutputTypeHTTP {
			logPipeline = kitlog.NewPipeline("http-output-pipeline").WithSecretKeyRef(mockBackend.HostSecretRef()).WithHTTPOutput()
		} else {
			logPipeline = kitlog.NewPipeline("custom-output-pipeline").WithCustomOutput(mockBackend.ExternalService.Host())
		}
		objs = append(objs, logPipeline.K8sObject())

		return objs
	}
	Context("When a logpipeline with HTTP output exists", Ordered, func() {
		const (
			mockBackendName = "log-receiver"
			mockNs          = "log-http-output"
			logProducerName = "log-producer-http-output" //#nosec G101 -- This is a false positive
		)

		BeforeAll(func() {
			k8sObjects := makeResources(mockNs, mockBackendName, logProducerName, OutputTypeHTTP)
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a log backend running", Label("operational"), func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: mockBackendName})
		})

		It("Should have a log producer running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: logProducerName})
		})

		It("Should verify end-to-end log delivery with http ", Label("operational"), func() {
			verifiers.LogsShouldBeDelivered(proxyClient, logProducerName, urls.MockBackendExport(mockBackendName))
		})
	})

	Context("When a logpipeline with custom output exists", Ordered, func() {
		const (
			mockBackendName = "log-receiver"
			mockNs          = "log-custom-output"
			logProducerName = "log-producer-custom-output" //#nosec G101 -- This is a false positive
		)

		BeforeAll(func() {
			k8sObjects := makeResources(mockNs, mockBackendName, logProducerName, OutputTypeCustom)
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a log backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: mockBackendName})
		})

		It("Should verify end-to-end log delivery with custom output", func() {
			verifiers.LogsShouldBeDelivered(proxyClient, logProducerName, urls.MockBackendExport(mockBackendName))
		})
	})
})
