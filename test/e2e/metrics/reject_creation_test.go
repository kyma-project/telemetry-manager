package metrics

import (
	"testing"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestRejectPipelineCreation(t *testing.T) {

	const (
		tlsCrdValidationError = "Can define either both 'cert' and 'key', or neither"
		notFoundError         = "not found"
	)

	var (
		label     = suite.LabelMisc
		backendNs = "backend"
	)

	serverCerts, clientCerts, err := testutils.NewCertBuilder(kitbackend.DefaultName, backendNs).Build()
	Expect(err).ToNot(HaveOccurred())

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithTLS(*serverCerts))

	tests := []struct {
		pipeline telemetryv1alpha1.MetricPipeline
		errorMsg string
	}{
		{
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("otlp-output-with-default-proto-and-path").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backend.Endpoint()),
					testutils.OTLPEndpointPath("/v1/mock/metrics"),
				).
				Build(),
		},
		{
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("otlp-output-with-grpc-proto-and-path").
				WithOTLPOutput(testutils.OTLPEndpoint(
					backend.Endpoint()),
					testutils.OTLPEndpointPath("/v1/mock/metrics"),
					testutils.OTLPProtocol("grpc"),
				).
				Build(),
		},
		{
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("otlp-output-basic-auth-secretref-missing-key").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backend.Endpoint()),
					testutils.OTLPBasicAuthFromSecret("name", "namespace", "", ""),
				).
				Build(),
		},
		{
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("otlp-output-tls-missing-key").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backend.Endpoint()),
					testutils.OTLPClientTLS(&telemetryv1alpha1.OTLPTLS{
						CA:   &telemetryv1alpha1.ValueType{Value: clientCerts.CaCertPem.String()},
						Cert: &telemetryv1alpha1.ValueType{Value: clientCerts.ClientCertPem.String()},
					}),
				).
				Build(),
			errorMsg: tlsCrdValidationError,
		},
	}
	for _, tc := range tests {
		t.Run(label, func(t *testing.T) {
			suite.RegisterTestCase(t, label)

			resources := []client.Object{&tc.pipeline}

			t.Cleanup(func() {
				Expect(kitk8s.DeleteObjects(resources...)).Should(MatchError(ContainSubstring(notFoundError)))
			})
			if len(tc.errorMsg) > 0 {
				Expect(kitk8s.CreateObjects(t, resources...)).Should(MatchError(ContainSubstring(tc.errorMsg)))
			} else {
				Expect(kitk8s.CreateObjects(t, resources...)).ShouldNot(Succeed())
			}
		})
	}

}
