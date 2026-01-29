package misc

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

func TestMTLSExpiredCert_OTel(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelLogsMisc, suite.LabelMTLS)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
	)

	serverCerts, clientCerts, err := testutils.NewCertBuilder(kitbackend.DefaultName, backendNs).
		WithAboutToExpireShortlyClientCert().
		Build()
	Expect(err).ToNot(HaveOccurred())

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithMTLS(*serverCerts))

	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(
			testutils.OTLPEndpoint(backend.EndpointHTTPS()),
			testutils.OTLPClientMTLSFromString(
				clientCerts.CaCertPem.String(),
				clientCerts.ClientCertPem.String(),
				clientCerts.ClientKeyPem.String(),
			)).
		Build()

	resources := []client.Object{
		&pipeline,
	}

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	// Initially, the certificate is about to expire in a short amount of time
	assert.LogPipelineHasCondition(t, pipelineName, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionTrue,
		Reason: conditions.ReasonTLSCertificateAboutToExpire,
	})

	assert.TelemetryHasState(t, operatorv1beta1.StateWarning)
	assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
		Type:   conditions.TypeLogComponentsHealthy,
		Status: metav1.ConditionTrue,
		Reason: conditions.ReasonTLSCertificateAboutToExpire,
	})

	// After certificate is expired, reconciliation should be triggered and status updated
	assert.LogPipelineHasCondition(t, pipelineName, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonTLSCertificateExpired,
	})

	assert.LogPipelineHasCondition(t, pipelineName, metav1.Condition{
		Type:   conditions.TypeFlowHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonSelfMonConfigNotGenerated,
	})

	assert.TelemetryHasState(t, operatorv1beta1.StateWarning)
	assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
		Type:   conditions.TypeLogComponentsHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonTLSCertificateExpired,
	})
}

func TestMTLSExpiredCert_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
	)

	serverCerts, clientCerts, err := testutils.NewCertBuilder(kitbackend.DefaultName, backendNs).
		WithAboutToExpireShortlyClientCert().
		Build()
	Expect(err).ToNot(HaveOccurred())

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithMTLS(*serverCerts))

	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithHTTPOutput(
			testutils.HTTPHost(backend.Host()),
			testutils.HTTPPort(backend.Port()),
			testutils.HTTPClientTLSFromString(
				clientCerts.CaCertPem.String(),
				clientCerts.ClientCertPem.String(),
				clientCerts.ClientKeyPem.String(),
			)).
		Build()

	resources := []client.Object{
		&pipeline,
	}

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	// Initially, the certificate is about to expire in a short amount of time
	assert.LogPipelineHasCondition(t, pipelineName, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionTrue,
		Reason: conditions.ReasonTLSCertificateAboutToExpire,
	})

	assert.TelemetryHasState(t, operatorv1beta1.StateWarning)
	assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
		Type:   conditions.TypeLogComponentsHealthy,
		Status: metav1.ConditionTrue,
		Reason: conditions.ReasonTLSCertificateAboutToExpire,
	})

	// After certificate is expired, reconciliation should be triggered and status updated
	assert.LogPipelineHasCondition(t, pipelineName, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonTLSCertificateExpired,
	})

	assert.LogPipelineHasCondition(t, pipelineName, metav1.Condition{
		Type:   conditions.TypeFlowHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonSelfMonConfigNotGenerated,
	})

	assert.TelemetryHasState(t, operatorv1beta1.StateWarning)
	assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
		Type:   conditions.TypeLogComponentsHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonTLSCertificateExpired,
	})
}
