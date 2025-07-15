package metrics

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMTLSCertKeyPairDontMatch(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetricsSetA)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
	)

	serverCertsDefault, clientCertsDefault, err := testutils.NewCertBuilder(kitbackend.DefaultName, backendNs).Build()
	Expect(err).ToNot(HaveOccurred())

	_, clientCertsCreatedAgain, err := testutils.NewCertBuilder(kitbackend.DefaultName, backendNs).Build()
	Expect(err).ToNot(HaveOccurred())

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithTLS(*serverCertsDefault))

	pipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(
			testutils.OTLPEndpoint(backend.Endpoint()),
			testutils.OTLPClientTLSFromString(
				clientCertsDefault.CaCertPem.String(),
				clientCertsDefault.ClientCertPem.String(),
				clientCertsCreatedAgain.ClientKeyPem.String(), // Use different key
			),
		).
		Build()

	resources := []client.Object{
		&pipeline,
	}

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(resources...))
	})
	Expect(kitk8s.CreateObjects(t, resources...)).Should(Succeed())

	assert.MetricPipelineHasCondition(t, pipelineName, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonTLSConfigurationInvalid,
	})

	assert.MetricPipelineHasCondition(t, pipelineName, metav1.Condition{
		Type:   conditions.TypeFlowHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonSelfMonConfigNotGenerated,
	})

	assert.TelemetryHasState(t, operatorv1alpha1.StateWarning)
	assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
		Type:   conditions.TypeMetricComponentsHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonTLSConfigurationInvalid,
	})
}
