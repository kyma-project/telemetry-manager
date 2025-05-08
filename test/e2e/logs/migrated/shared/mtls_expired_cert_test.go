package shared

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMTLSExpiredCert_OTel(t *testing.T) {
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
				backendName  = backend.DefaultName
			)

			expiredServerCerts, expiredClientCerts, err := testutils.NewCertBuilder(backendName, backendNs).
				WithExpiredClientCert().
				Build()
			Expect(err).ToNot(HaveOccurred())

			backend := backend.New(backendNs, backend.SignalTypeLogsOTel, backend.WithTLS(*expiredServerCerts))

			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.inputBuilder()).
				WithOTLPOutput(
					testutils.OTLPEndpoint(backend.Endpoint()),
					testutils.OTLPClientTLSFromString(
						expiredClientCerts.CaCertPem.String(),
						expiredClientCerts.ClientCertPem.String(),
						expiredClientCerts.ClientKeyPem.String(),
					)).
				Build()

			var resources []client.Object
			resources = append(resources,
				&pipeline,
			)

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
			})
			Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

			assert.LogPipelineHasCondition(t.Context(), suite.K8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonTLSCertificateExpired,
			})

			assert.LogPipelineHasCondition(t.Context(), suite.K8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonSelfMonConfigNotGenerated,
			})

			assert.TelemetryHasState(t.Context(), suite.K8sClient, operatorv1alpha1.StateWarning)
			assert.TelemetryHasCondition(t.Context(), suite.K8sClient, metav1.Condition{
				Type:   conditions.TypeLogComponentsHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonTLSCertificateExpired,
			})
		})
	}
}

func TestMTLSExpiredCert_FluentBit(t *testing.T) {
	RegisterTestingT(t)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		backendName  = backend.DefaultName
	)

	expiredServerCerts, expiredClientCerts, err := testutils.NewCertBuilder(backendName, backendNs).
		WithExpiredClientCert().
		Build()
	Expect(err).ToNot(HaveOccurred())

	backend := backend.New(backendNs, backend.SignalTypeLogsFluentBit, backend.WithTLS(*expiredServerCerts))

	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithHTTPOutput(
			testutils.HTTPHost(backend.Host()),
			testutils.HTTPPort(backend.Port()),
			testutils.HTTPClientTLSFromString(
				expiredClientCerts.CaCertPem.String(),
				expiredClientCerts.ClientCertPem.String(),
				expiredClientCerts.ClientKeyPem.String(),
			)).
		Build()

	var resources []client.Object
	resources = append(resources,
		&pipeline,
	)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	assert.LogPipelineHasCondition(t.Context(), suite.K8sClient, pipelineName, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonTLSCertificateExpired,
	})

	assert.LogPipelineHasCondition(t.Context(), suite.K8sClient, pipelineName, metav1.Condition{
		Type:   conditions.TypeFlowHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonSelfMonConfigNotGenerated,
	})

	assert.TelemetryHasState(t.Context(), suite.K8sClient, operatorv1alpha1.StateWarning)
	assert.TelemetryHasCondition(t.Context(), suite.K8sClient, metav1.Condition{
		Type:   conditions.TypeLogComponentsHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonTLSCertificateExpired,
	})
}
