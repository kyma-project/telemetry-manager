package metrics

import (
	"context"
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

func TestMTLSMissingKey(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetricsSetB)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
	)

	serverCerts, clientCerts, err := testutils.NewCertBuilder(kitbackend.DefaultName, backendNs).Build()
	Expect(err).ToNot(HaveOccurred())

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithTLS(*serverCerts))

	pipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(
			testutils.OTLPEndpoint(backend.Endpoint()),
			testutils.OTLPClientTLS(&telemetryv1alpha1.OTLPTLS{
				CA:   &telemetryv1alpha1.ValueType{Value: clientCerts.CaCertPem.String()},
				Cert: &telemetryv1alpha1.ValueType{Value: clientCerts.ClientCertPem.String()},
			}),
		).
		Build()

	resources := []client.Object{
		&pipeline,
	}

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(context.Background(), resources...)).Should(MatchError(ContainSubstring(notFoundError))) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(MatchError(ContainSubstring(tlsCrdValidationError)))
}
