package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/oauth2mock"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestOAuth2(t *testing.T) {
	tests := []struct {
		name                string
		labels              []string
		inputBuilder        func(includeNs string) telemetryv1beta1.LogPipelineInput
		logGeneratorBuilder func(ns string) client.Object
		resourceName        types.NamespacedName
	}{
		{
			name:   suite.LabelLogAgent,
			labels: []string{suite.LabelLogAgent, suite.LabelOAuth2},
			inputBuilder: func(includeNs string) telemetryv1beta1.LogPipelineInput {
				return testutils.BuildLogPipelineRuntimeInput(testutils.IncludeNamespaces(includeNs))
			},
			logGeneratorBuilder: func(ns string) client.Object {
				return stdoutloggen.NewDeployment(ns).K8sObject()
			},
			resourceName: kitkyma.LogAgentName,
		},
		{
			name:   suite.LabelLogGateway,
			labels: []string{suite.LabelLogGateway, suite.LabelOAuth2},
			inputBuilder: func(includeNs string) telemetryv1beta1.LogPipelineInput {
				return testutils.BuildLogPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
			logGeneratorBuilder: func(ns string) client.Object {
				return telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeLogs).K8sObject()
			},
			resourceName: kitkyma.OTLPGatewayName,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			suite.SetupTest(t, tc.labels...)

			var (
				uniquePrefix = unique.Prefix(tc.name)
				pipelineName = uniquePrefix()
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			oauth2server := oauth2mock.New(backendNs)

			serverCerts, _, err := testutils.NewCertBuilder(kitbackend.DefaultName, backendNs).Build()
			Expect(err).ToNot(HaveOccurred())

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel,
				kitbackend.WithTLS(*serverCerts),
				kitbackend.WithOIDCAuth(oauth2server.IssuerURL(), oauth2server.Audience()),
			)

			oauth2Secret := kitk8sobjects.NewOpaqueSecret("oauth2", kitkyma.DefaultNamespaceName,
				kitk8sobjects.WithStringData("client-id", "the-mock-does-not-verify"),
				kitk8sobjects.WithStringData("client-secret", "the-mock-does-not-verify"),
				kitk8sobjects.WithStringData("token-url", oauth2server.TokenEndpoint()),
			)

			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.inputBuilder(genNs)).
				WithOTLPOutput(
					testutils.OTLPEndpoint(backend.EndpointHTTPS()),
					testutils.OTLPOAuth2(
						testutils.OAuth2ClientIDFromSecret(oauth2Secret.Name(), oauth2Secret.Namespace(), "client-id"),
						testutils.OAuth2ClientSecretFromSecret(oauth2Secret.Name(), oauth2Secret.Namespace(), "client-secret"),
						testutils.OAuth2TokenURLFromSecret(oauth2Secret.Name(), oauth2Secret.Namespace(), "token-url"),
						testutils.OAuth2Params(map[string]string{"grant_type": "client_credentials"}),
					),
					testutils.OTLPClientTLSFromString(serverCerts.CaCertPem.String()),
				).
				Build()

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				oauth2Secret.K8sObject(),
				&pipeline,
				tc.logGeneratorBuilder(genNs),
			}

			resources = append(resources, oauth2server.K8sObjects()...)
			resources = append(resources, backend.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.DeploymentReady(t, oauth2server.NamespacedName())
			assert.BackendReachable(t, backend)

			assert.DaemonSetReady(t, tc.resourceName)

			assert.OTelLogPipelineHealthy(t, pipelineName)
			assert.OTelLogsFromNamespaceDelivered(t, backend, genNs)
		})
	}
}
