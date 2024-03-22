package e2e

import (
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	. "github.com/onsi/ginkgo/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Telemetry Self Monitor", Ordered, func() {
	const (
		mockBackendName = "log-receiver"
		mockNs          = "log-http-output"
		logProducerName = "log-producer-http-output" //#nosec G101 -- This is a false positive
		pipelineName    = "http-output-pipeline"
	)
	var telemetryExportURL string

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeLogs, backend.WithPersistentHostSecret(isOperational()))
		mockLogProducer := loggen.New(logProducerName, mockNs)
		objs = append(objs, mockBackend.K8sObjects()...)
		objs = append(objs, mockLogProducer.K8sObject(kitk8s.WithLabel("app", "logging-test-with-selfmon")))
		telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)

		logPipeline := kitk8s.NewLogPipelineV1Alpha1(pipelineName).
			WithSecretKeyRef(mockBackend.HostSecretRefV1Alpha1()).
			WithHTTPOutput().
			Persistent(isOperational())
		objs = append(objs, logPipeline.K8sObject())

		return objs
	}

	Context("When a log pipeline with HTTP output exists", Ordered, func() {
		It("should have a running self-monitor")
	})

})
