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
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMTLSAboutToExpireCert_OTel(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		name                string
		inputBuilder        func() telemetryv1alpha1.LogPipelineInput
		logGeneratorBuilder func(namespace string) client.Object
		expectAgent         bool
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
			logGeneratorBuilder: func(namespace string) client.Object {
				return loggen.New(namespace).K8sObject()
			},
			expectAgent: true,
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
			logGeneratorBuilder: func(namespace string) client.Object {
				return telemetrygen.NewDeployment(namespace, telemetrygen.SignalTypeLogs).K8sObject()
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				uniquePrefix = unique.Prefix(tc.name)
				pipelineName = uniquePrefix("pipeline")
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
				backendName  = backend.DefaultName
			)

			serverCerts, clientCerts, err := testutils.NewCertBuilder(backendName, backendNs).
				WithAboutToExpireClientCert().
				Build()
			Expect(err).ToNot(HaveOccurred())

			backend := backend.New(backendNs, backend.SignalTypeLogsOTel, backend.WithTLS(*serverCerts))
			backendExportURL := backend.ExportURL(suite.ProxyClient)

			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.inputBuilder()).
				WithOTLPOutput(
					testutils.OTLPEndpoint(backend.Endpoint()),
					testutils.OTLPClientTLSFromString(
						clientCerts.CaCertPem.String(),
						clientCerts.ClientCertPem.String(),
						clientCerts.ClientKeyPem.String(),
					)).
				Build()

			var resources []client.Object
			resources = append(resources,
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(genNs).K8sObject(),
				&pipeline,
				tc.logGeneratorBuilder(genNs),
			)
			resources = append(resources, backend.K8sObjects()...)

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
			})
			Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

			assert.DeploymentReady(t.Context(), suite.K8sClient, kitkyma.LogGatewayName)
			assert.DeploymentReady(t.Context(), suite.K8sClient, backend.NamespacedName())

			if tc.expectAgent {
				assert.DaemonSetReady(suite.Ctx, suite.K8sClient, kitkyma.LogAgentName)
			}

			assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineName)
			assert.LogPipelineHasCondition(suite.Ctx, suite.K8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonTLSCertificateAboutToExpire,
			})

			assert.TelemetryHasState(suite.Ctx, suite.K8sClient, operatorv1alpha1.StateWarning)
			assert.TelemetryHasCondition(suite.Ctx, suite.K8sClient, metav1.Condition{
				Type:   conditions.TypeLogComponentsHealthy,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonTLSCertificateAboutToExpire,
			})

			assert.OTelLogsFromNamespaceDelivered(suite.ProxyClient, genNs, backendExportURL)
		})
	}
}

func TestMTLSAboutToExpireCert_FluentBit(t *testing.T) {
	RegisterTestingT(t)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix("pipeline")
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
		backendName  = backend.DefaultName
	)

	serverCerts, clientCerts, err := testutils.NewCertBuilder(backendName, backendNs).
		WithAboutToExpireClientCert().
		Build()
	Expect(err).ToNot(HaveOccurred())

	backend := backend.New(backendNs, backend.SignalTypeLogsFluentBit, backend.WithTLS(*serverCerts))
	backendExportURL := backend.ExportURL(suite.ProxyClient)

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

	var resources []client.Object
	resources = append(resources,
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&pipeline,
		loggen.New(genNs).K8sObject(),
	)
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	assert.DeploymentReady(t.Context(), suite.K8sClient, backend.NamespacedName())
	assert.DaemonSetReady(suite.Ctx, suite.K8sClient, kitkyma.FluentBitDaemonSetName)

	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineName)
	assert.LogPipelineHasCondition(suite.Ctx, suite.K8sClient, pipelineName, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionTrue,
		Reason: conditions.ReasonTLSCertificateAboutToExpire,
	})

	assert.TelemetryHasState(suite.Ctx, suite.K8sClient, operatorv1alpha1.StateWarning)
	assert.TelemetryHasCondition(suite.Ctx, suite.K8sClient, metav1.Condition{
		Type:   conditions.TypeLogComponentsHealthy,
		Status: metav1.ConditionTrue,
		Reason: conditions.ReasonTLSCertificateAboutToExpire,
	})

	assert.FluentBitLogsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, genNs)
}
