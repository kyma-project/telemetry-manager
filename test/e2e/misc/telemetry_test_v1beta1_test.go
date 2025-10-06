package misc

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestTelemetryV1Beta1(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelExperimental)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("trace-backend")
		genNs        = uniquePrefix("gen")

		traceGRPCEndpoint = "http://telemetry-otlp-traces.kyma-system:4317"
		traceHTTPEndpoint = "http://telemetry-otlp-traces.kyma-system:4318"

		metricGRPCEndpoint = "http://telemetry-otlp-metrics.kyma-system:4317"
		metricHTTPEndpoint = "http://telemetry-otlp-metrics.kyma-system:4318"

		logGRPCEndpoint = "http://telemetry-otlp-logs.kyma-system:4317"
		logHTTPEndpoint = "http://telemetry-otlp-logs.kyma-system:4318"
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeTraces)

	tracePipeline := testutils.NewTracePipelineBuilder().WithName(pipelineName).Build()
	metricPipeline := testutils.NewMetricPipelineBuilder().WithName(pipelineName).Build()
	logPipeline := testutils.NewLogPipelineBuilder().WithName(pipelineName).WithOTLPInput(true).WithOTLPOutput().Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&tracePipeline,
		&metricPipeline,
		&logPipeline,
	}

	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	Eventually(func(g Gomega) {
		var telemetry operatorv1beta1.Telemetry
		g.Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.TelemetryName, &telemetry)).Should(Succeed())

		g.Expect(telemetry.Status.Endpoints.Logs).ShouldNot(BeNil())
		g.Expect(telemetry.Status.Endpoints.Logs.GRPC).Should(Equal(logGRPCEndpoint))
		g.Expect(telemetry.Status.Endpoints.Logs.HTTP).Should(Equal(logHTTPEndpoint))

		g.Expect(telemetry.Status.Endpoints.Traces).ShouldNot(BeNil())
		g.Expect(telemetry.Status.Endpoints.Traces.GRPC).Should(Equal(traceGRPCEndpoint))
		g.Expect(telemetry.Status.Endpoints.Traces.HTTP).Should(Equal(traceHTTPEndpoint))

		g.Expect(telemetry.Status.Endpoints.Metrics).ShouldNot(BeNil())
		g.Expect(telemetry.Status.Endpoints.Metrics.GRPC).Should(Equal(metricGRPCEndpoint))
		g.Expect(telemetry.Status.Endpoints.Metrics.HTTP).Should(Equal(metricHTTPEndpoint))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())

	assertValidatingWebhookConfiguration()
	assertWebhookCA()
	assertWebhookSecretReconcilation()
}
