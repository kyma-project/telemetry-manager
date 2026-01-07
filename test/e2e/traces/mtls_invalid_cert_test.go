package traces

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMTLSInvalidCert(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelTraces, suite.LabelMTLS)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
	)

	invalidServerCerts, invalidClientCerts, err := testutils.NewCertBuilder(kitbackend.DefaultName, backendNs).
		WithInvalidClientCert().
		Build()
	Expect(err).ToNot(HaveOccurred())

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeTraces, kitbackend.WithMTLS(*invalidServerCerts))

	pipeline := testutils.NewTracePipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(
			testutils.OTLPEndpoint(backend.EndpointHTTPS()),
			testutils.OTLPClientMTLSFromString(
				invalidClientCerts.CaCertPem.String(),
				invalidClientCerts.ClientCertPem.String(),
				invalidClientCerts.ClientKeyPem.String(),
			),
		).
		Build()

	resources := []client.Object{
		&pipeline,
	}

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.TracePipelineHasCondition(t, pipelineName, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonTLSConfigurationInvalid,
	})

	assert.TracePipelineHasCondition(t, pipelineName, metav1.Condition{
		Type:   conditions.TypeFlowHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonSelfMonConfigNotGenerated,
	})

	assert.TelemetryHasState(t, operatorv1beta1.StateWarning)
	assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
		Type:   conditions.TypeTraceComponentsHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonTLSConfigurationInvalid,
	})
}
