package shared

import (
	"context"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

func TestDisabledOTLPInput_OTel(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelLogGateway)

	var (
		uniquePrefix        = unique.Prefix()
		pipelineAgentName   = uniquePrefix("agent")
		pipelineGatewayName = uniquePrefix("gateway")
		backendNs           = uniquePrefix("backend")

		genNs          = uniquePrefix("gen")
		telemetryGenNs = uniquePrefix("tel-gen")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)

	pipelineAgent := testutils.NewLogPipelineBuilder().
		WithName(pipelineAgentName).
		WithApplicationInput(true, []testutils.ExtendedNamespaceSelectorOptions{testutils.ExtIncludeNamespaces(genNs)}...).
		WithOTLPOutput(
			testutils.OTLPEndpoint("telemetry-otlp-logs.kyma-system:4317"),
			testutils.OTLPClientTLS( &telemetryv1alpha1.OTLPTLS{
				Insecure: true,
			}),
		).
		Build()

	pipelineGateway := testutils.NewLogPipelineBuilder().
		WithName(pipelineGatewayName).
		WithApplicationInput(false).
		WithOTLPInput(false).
		WithOTLPOutput(
			testutils.OTLPEndpoint(backend.Endpoint()),
		).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(telemetryGenNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&pipelineAgent,
		&pipelineGateway,
		loggen.New(genNs).K8sObject(),
		telemetrygen.NewPod(telemetryGenNs, telemetrygen.SignalTypeLogs).K8sObject(),
	}

	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.LogAgentName)
	assert.DeploymentReady(t.Context(), suite.K8sClient, kitkyma.LogGatewayName)
	assert.DeploymentReady(t.Context(), suite.K8sClient, types.NamespacedName{Name: kitbackend.DefaultName, Namespace: backendNs})
	assert.OTelLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineAgentName)
	assert.OTelLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineGatewayName)

	assert.OTelLogsFromNamespaceDelivered(t.Context(), backend, genNs)
	assert.OTelLogsFromNamespaceNotDelivered(t.Context(), backend, telemetryGenNs)

}
