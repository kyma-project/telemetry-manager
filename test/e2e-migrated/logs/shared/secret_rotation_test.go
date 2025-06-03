package shared

import (
	"context"
	"testing"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestSecretRotation_OTel(t *testing.T) {
	tests := []struct {
		label               string
		inputBuilder        func(includeNs string) telemetryv1alpha1.LogPipelineInput
		logGeneratorBuilder func(ns string) client.Object
		expectAgent         bool
	}{
		{
			label: suite.LabelLogAgent,
			inputBuilder: func(includeNs string) telemetryv1alpha1.LogPipelineInput {
				return testutils.BuildLogPipelineApplicationInput(testutils.ExtIncludeNamespaces(includeNs))
			},
			logGeneratorBuilder: func(ns string) client.Object {
				return stdloggen.NewDeployment(ns).K8sObject()
			},
			expectAgent: true,
		},
		{
			label: suite.LabelLogGateway,
			inputBuilder: func(includeNs string) telemetryv1alpha1.LogPipelineInput {
				return testutils.BuildLogPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
			logGeneratorBuilder: func(ns string) client.Object {
				return telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeLogs).K8sObject()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label)

			const endpointKey = "logs-endpoint"

			var (
				uniquePrefix = unique.Prefix(tc.label)
				pipelineName = uniquePrefix("pipeline")
				genNs        = uniquePrefix("gen")
				backendNs    = uniquePrefix("backend")
			)

			// Initially, create a secret with an incorrect endpoint
			secret := kitk8s.NewOpaqueSecret("rotation", genNs, kitk8s.WithStringData(endpointKey, "http://localhost:4000"))

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)

			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.inputBuilder(genNs)).
				WithOTLPOutput(testutils.OTLPEndpointFromSecret(
					secret.Name(),
					secret.Namespace(),
					endpointKey,
				)).
				Build()

			resources := []client.Object{
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(genNs).K8sObject(),
				&pipeline,
				tc.logGeneratorBuilder(genNs),
				secret.K8sObject(),
			}
			resources = append(resources, backend.K8sObjects()...)

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
			})
			require.NoError(t, kitk8s.CreateObjects(t.Context(), resources...))

			assert.DeploymentReady(t.Context(), backend.NamespacedName())
			assert.DeploymentReady(t.Context(), kitkyma.LogGatewayName)

			if tc.expectAgent {
				assert.DaemonSetReady(t.Context(), kitkyma.LogAgentName)
			}

			assert.OTelLogPipelineHealthy(t, pipelineName)

			// Initially, the logs should not be delivered due to the incorrect endpoint in the secret
			assert.OTelLogsFromNamespaceNotDelivered(t, backend, genNs)

			// Update the secret to have the correct backend endpoint
			secret.UpdateSecret(kitk8s.WithStringData(endpointKey, backend.Endpoint()))
			require.NoError(t, kitk8s.UpdateObjects(t.Context(), secret.K8sObject()))

			assert.DeploymentReady(t.Context(), backend.NamespacedName())
			assert.DeploymentReady(t.Context(), kitkyma.LogGatewayName)

			if tc.expectAgent {
				assert.DaemonSetReady(t.Context(), kitkyma.LogAgentName)
			}

			assert.OTelLogPipelineHealthy(t, pipelineName)

			// After updating the secret, the logs should be delivered
			assert.OTelLogsFromNamespaceDelivered(t, backend, genNs)
		})
	}
}
