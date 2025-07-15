package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

const (
	tlsCrdValidationError = "Can define either both 'cert' and 'key', or neither"
	notFoundError         = "not found"
)

func TestMTLSMissingKey_OTel(t *testing.T) {
	tests := []struct {
		label string
		input telemetryv1alpha1.LogPipelineInput
	}{
		{
			label: suite.LabelLogAgent,
			input: testutils.BuildLogPipelineApplicationInput(),
		},
		{
			label: suite.LabelLogGateway,
			input: testutils.BuildLogPipelineOTLPInput(),
		},
	}
	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label)

			var (
				uniquePrefix = unique.Prefix(tc.label)
				pipelineName = uniquePrefix()
				backendNs    = uniquePrefix("backend")
			)

			serverCerts, clientCerts, err := testutils.NewCertBuilder(kitbackend.DefaultName, backendNs).Build()
			Expect(err).ToNot(HaveOccurred())

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithTLS(*serverCerts))

			pipelineMissingKey := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.input).
				WithOTLPOutput(
					testutils.OTLPEndpoint(backend.Endpoint()),
					testutils.OTLPClientTLS(&telemetryv1alpha1.OTLPTLS{
						Cert: &telemetryv1alpha1.ValueType{Value: clientCerts.ClientCertPem.String()},
					}),
				).Build()

			resources := []client.Object{
				&pipelineMissingKey,
			}

			t.Cleanup(func() {
				Expect(kitk8s.DeleteObjects(resources...)).Should(MatchError(ContainSubstring(notFoundError)))
			})
			Expect(kitk8s.CreateObjects(t, resources...)).Should(MatchError(ContainSubstring(tlsCrdValidationError)))
		})
	}
}

func TestMTLSMissingKey_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
	)

	serverCerts, clientCerts, err := testutils.NewCertBuilder(kitbackend.DefaultName, backendNs).Build()
	Expect(err).ToNot(HaveOccurred())

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithTLS(*serverCerts))

	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithHTTPOutput(
			testutils.HTTPHost(backend.Host()),
			testutils.HTTPPort(backend.Port()),
			testutils.HTTPClientTLS(telemetryv1alpha1.LogPipelineOutputTLS{
				Cert: &telemetryv1alpha1.ValueType{Value: clientCerts.ClientCertPem.String()},
			}),
		).
		Build()

	resources := []client.Object{
		&pipeline,
	}

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(resources...)).Should(MatchError(ContainSubstring(notFoundError)))
	})
	Expect(kitk8s.CreateObjects(t, resources...)).Should(MatchError(ContainSubstring(tlsCrdValidationError)))
}
