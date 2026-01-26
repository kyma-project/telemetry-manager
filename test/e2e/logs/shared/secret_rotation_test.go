package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
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

func TestSecretRotation_OTel(t *testing.T) {
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
			suite.RegisterTestCase(t, tc.label)

			const (
				endpointKey   = "logs-endpoint"
				endpointValue = "http://localhost:4000"
			)

			var (
				uniquePrefix = unique.Prefix(tc.label)
				pipelineName = uniquePrefix()
				secretName   = uniquePrefix()
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)

			// Initially, create a secret with an incorrect endpoint
			secret := kitk8sobjects.NewOpaqueSecret(secretName, kitkyma.DefaultNamespaceName, kitk8sobjects.WithStringData(endpointKey, endpointValue))

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
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				&pipeline,
				tc.logGeneratorBuilder(genNs),
				secret.K8sObject(),
			}
			resources = append(resources, backend.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backend)
			assert.DeploymentReady(t, kitkyma.LogGatewayName)

			if tc.expectAgent {
				assert.DaemonSetReady(t, kitkyma.LogAgentName)
			}

			assert.OTelLogPipelineHealthy(t, pipelineName)
			assert.OTelLogsFromNamespaceNotDelivered(t, backend, genNs)

			// Update the secret to have the correct backend endpoint
			secret.UpdateSecret(kitk8sobjects.WithStringData(endpointKey, backend.EndpointHTTP()))
			Expect(kitk8s.UpdateObjects(t, secret.K8sObject())).To(Succeed())

			assert.DeploymentReady(t, kitkyma.LogGatewayName)

			if tc.expectAgent {
				assert.DaemonSetReady(t, kitkyma.LogAgentName)
			}

			assert.OTelLogPipelineHealthy(t, pipelineName)
			assert.OTelLogsFromNamespaceDelivered(t, backend, genNs)
		})
	}
}

func TestSecretRotation_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	const (
		hostKey   = "logs-host"
		hostValue = "localhost"
	)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		secretName   = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit)

	// Initially, create a secret with an incorrect host
	secret := kitk8sobjects.NewOpaqueSecret(secretName, kitkyma.DefaultNamespaceName, kitk8sobjects.WithStringData(hostKey, hostValue))

	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithHTTPOutput(
			testutils.HTTPHostFromSecret(
				secret.Name(),
				secret.Namespace(),
				hostKey,
			),
			testutils.HTTPPort(backend.Port()),
		).
		Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(genNs).K8sObject(),
		stdoutloggen.NewDeployment(genNs).K8sObject(),
		&pipeline,
		secret.K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
	assert.FluentBitLogPipelineHealthy(t, pipelineName)
	assert.FluentBitLogsFromNamespaceNotDelivered(t, backend, genNs)

	// Update the secret to have the correct backend host
	secret.UpdateSecret(kitk8sobjects.WithStringData(hostKey, backend.Host()))
	Expect(kitk8s.UpdateObjects(t, secret.K8sObject())).To(Succeed())

	assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
	assert.FluentBitLogPipelineHealthy(t, pipelineName)
	assert.FluentBitLogsFromNamespaceDelivered(t, backend, genNs)
}
