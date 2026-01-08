package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMTLSAboutToExpireCert_OTel(t *testing.T) {
	tests := []struct {
		label               string
		inputBuilder        func(includeNs string) telemetryv1beta1.LogPipelineInput
		logGeneratorBuilder func(ns string) client.Object
		expectAgent         bool
	}{
		{
			label: suite.LabelLogAgent,
			inputBuilder: func(includeNs string) telemetryv1beta1.LogPipelineInput {
				return testutils.BuildLogPipelineRuntimeInput(testutils.IncludeNamespaces(includeNs))
			},
			logGeneratorBuilder: func(ns string) client.Object {
				return stdoutloggen.NewDeployment(ns).K8sObject()
			},
			expectAgent: true,
		},
		{
			label: suite.LabelLogGateway,
			inputBuilder: func(includeNs string) telemetryv1beta1.LogPipelineInput {
				return testutils.BuildLogPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
			logGeneratorBuilder: func(ns string) client.Object {
				return telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeLogs).K8sObject()
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label, suite.LabelMTLS)

			var (
				uniquePrefix = unique.Prefix(tc.label)
				pipelineName = uniquePrefix()
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			serverCerts, clientCerts, err := testutils.NewCertBuilder(kitbackend.DefaultName, backendNs).
				WithAboutToExpireClientCert().
				Build()
			Expect(err).ToNot(HaveOccurred())

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithMTLS(*serverCerts))

			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.inputBuilder(genNs)).
				WithOTLPOutput(
					testutils.OTLPEndpoint(backend.EndpointHTTPS()),
					testutils.OTLPClientMTLSFromString(
						clientCerts.CaCertPem.String(),
						clientCerts.ClientCertPem.String(),
						clientCerts.ClientKeyPem.String(),
					)).
				Build()

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				&pipeline,
				tc.logGeneratorBuilder(genNs),
			}
			resources = append(resources, backend.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backend)
			assert.DeploymentReady(t, kitkyma.LogGatewayName)

			if tc.expectAgent {
				assert.DaemonSetReady(t, kitkyma.LogAgentName)
			}

			assert.FluentBitLogPipelineHealthy(t, pipelineName)
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

			assert.OTelLogsFromNamespaceDelivered(t, backend, genNs)
		})
	}
}

func TestMTLSAboutToExpireCert_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
	)

	serverCerts, clientCerts, err := testutils.NewCertBuilder(kitbackend.DefaultName, backendNs).
		WithAboutToExpireClientCert().
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
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(genNs).K8sObject(),
		&pipeline,
		stdoutloggen.NewDeployment(genNs).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
	assert.FluentBitLogPipelineHealthy(t, pipelineName)

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

	assert.FluentBitLogsFromNamespaceDelivered(t, backend, genNs)
}
