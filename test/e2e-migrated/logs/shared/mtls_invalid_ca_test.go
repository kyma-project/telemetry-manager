package shared

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMTLSInvalidCA_OTel(t *testing.T) {
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

			invalidServerCerts, invalidClientCerts, err := testutils.NewCertBuilder(kitbackend.DefaultName, backendNs).
				WithInvalidCA().
				Build()
			Expect(err).ToNot(HaveOccurred())

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithTLS(*invalidServerCerts))

			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.input).
				WithOTLPOutput(
					testutils.OTLPEndpoint(backend.Endpoint()),
					testutils.OTLPClientTLSFromString(
						invalidClientCerts.CaCertPem.String(),
						invalidClientCerts.ClientCertPem.String(),
						invalidClientCerts.ClientKeyPem.String(),
					)).
				Build()

			var resources []client.Object
			resources = append(resources,
				&pipeline,
			)

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
			})
			Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

			assert.LogPipelineHasCondition(t, pipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonTLSConfigurationInvalid,
			})

			assert.LogPipelineHasCondition(t, pipelineName, metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonSelfMonConfigNotGenerated,
			})

			assert.TelemetryHasState(t.Context(), operatorv1alpha1.StateWarning)
			assert.TelemetryHasCondition(t.Context(), suite.K8sClient, metav1.Condition{
				Type:   conditions.TypeLogComponentsHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonTLSConfigurationInvalid,
			})
		})
	}
}

func TestMTLSInvalidCA_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
	)

	invalidServerCerts, invalidClientCerts, err := testutils.NewCertBuilder(kitbackend.DefaultName, backendNs).
		WithInvalidCA().
		Build()
	Expect(err).ToNot(HaveOccurred())

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithTLS(*invalidServerCerts))

	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithHTTPOutput(
			testutils.HTTPHost(backend.Host()),
			testutils.HTTPPort(backend.Port()),
			testutils.HTTPClientTLSFromString(
				invalidClientCerts.CaCertPem.String(),
				invalidClientCerts.ClientCertPem.String(),
				invalidClientCerts.ClientKeyPem.String(),
			)).
		Build()

	var resources []client.Object
	resources = append(resources,
		&pipeline,
	)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

	assert.LogPipelineHasCondition(t, pipelineName, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonTLSConfigurationInvalid,
	})

	assert.LogPipelineHasCondition(t, pipelineName, metav1.Condition{
		Type:   conditions.TypeFlowHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonSelfMonConfigNotGenerated,
	})

	assert.TelemetryHasState(t.Context(), operatorv1alpha1.StateWarning)
	assert.TelemetryHasCondition(t.Context(), suite.K8sClient, metav1.Condition{
		Type:   conditions.TypeLogComponentsHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonTLSConfigurationInvalid,
	})
}
