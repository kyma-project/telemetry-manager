package shared

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

const (
	tlsCrdValidationError = "Can define either both 'cert' and 'key', or neither"
	notFoundError         = "not found"
)

func TestMTLSMissingValues_OTel(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		name         string
		inputBuilder func() telemetryv1alpha1.LogPipelineInput
	}{
		{
			name: "agent",
			inputBuilder: func() telemetryv1alpha1.LogPipelineInput {
				return telemetryv1alpha1.LogPipelineInput{
					Application: &telemetryv1alpha1.LogPipelineApplicationInput{
						Enabled: ptr.To(true),
					},
				}
			},
		},
		{
			name: "gateway",
			inputBuilder: func() telemetryv1alpha1.LogPipelineInput {
				return telemetryv1alpha1.LogPipelineInput{
					Application: &telemetryv1alpha1.LogPipelineApplicationInput{
						Enabled: ptr.To(false),
					},
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				uniquePrefix = unique.Prefix(tc.name)
				pipelineName = uniquePrefix()
				backendNs    = uniquePrefix("backend")
			)

			serverCerts, clientCerts, err := testutils.NewCertBuilder(backend.DefaultName, backendNs).Build()
			Expect(err).ToNot(HaveOccurred())

			backend := backend.New(backendNs, backend.SignalTypeLogsOTel, backend.WithTLS(*serverCerts))

			pipelineMissingKey := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.inputBuilder()).
				WithOTLPOutput(
					testutils.OTLPEndpoint(backend.Endpoint()),
					testutils.OTLPClientTLS(&telemetryv1alpha1.OTLPTLS{
						Cert: &telemetryv1alpha1.ValueType{Value: clientCerts.ClientCertPem.String()},
					}),
				).Build()

			var resources []client.Object
			resources = append(resources,
				&pipelineMissingKey,
			)

			t.Cleanup(func() {
				Expect(kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)).Should(MatchError(ContainSubstring(notFoundError))) //nolint:usetesting // Remove ctx from DeleteObjects
			})
			Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(MatchError(ContainSubstring(tlsCrdValidationError)))
		})
	}
}

func TestMTLSMissingValues_FluentBit(t *testing.T) {
	RegisterTestingT(t)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
	)

	serverCerts, clientCerts, err := testutils.NewCertBuilder(backend.DefaultName, backendNs).Build()
	Expect(err).ToNot(HaveOccurred())

	backend := backend.New(backendNs, backend.SignalTypeLogsFluentBit, backend.WithTLS(*serverCerts))

	pipelineMissingKey := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithHTTPOutput(
			testutils.HTTPHost(backend.Host()),
			testutils.HTTPPort(backend.Port()),
			testutils.HTTPClientTLS(telemetryv1alpha1.LogPipelineOutputTLS{
				Cert: &telemetryv1alpha1.ValueType{Value: clientCerts.ClientCertPem.String()},
			}),
		).
		Build()

	var resources []client.Object
	resources = append(resources,
		&pipelineMissingKey,
	)

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)).Should(MatchError(ContainSubstring(notFoundError))) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(MatchError(ContainSubstring(tlsCrdValidationError)))

}
