package migrated

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdloggen"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestMain(m *testing.M) {
	suite.BeforeSuiteFunc()
	m.Run()
	suite.AfterSuiteFunc()
}

func TestOTelLogPipeline_ServiceNameEnrichment(t *testing.T) {
	RegisterTestingT(t)
	skipIfDoesNotMatchLabel(t, "logs")

	const (
		appLabelName  = "app"
		appLabelValue = "workload"
	)

	tests := []struct {
		name                 string
		logPipelineInputFunc func(includeNs []string, excludeNs []string) telemetryv1alpha1.LogPipelineInput
		logProducerFunc      func(deploymentName, namespace string) client.Object
	}{
		{
			name: "otlp",
			logPipelineInputFunc: func(includeNs []string, excludeNs []string) telemetryv1alpha1.LogPipelineInput {
				return telemetryv1alpha1.LogPipelineInput{
					Application: &telemetryv1alpha1.LogPipelineApplicationInput{Enabled: ptr.To(false)},
					OTLP:        &telemetryv1alpha1.OTLPInput{Disabled: false},
				}
			},
			logProducerFunc: func(deploymentName, namespace string) client.Object {
				podSpecWithUndefinedService := telemetrygen.PodSpec(telemetrygen.SignalTypeLogs, telemetrygen.WithServiceName(""))
				return kitk8sobjects.NewDeployment(deploymentName, namespace).
					WithLabel(appLabelName, appLabelValue).
					WithPodSpec(podSpecWithUndefinedService).
					K8sObject()
			},
		},
		{
			name: "application",
			logPipelineInputFunc: func(includeNs []string, excludeNs []string) telemetryv1alpha1.LogPipelineInput {
				return telemetryv1alpha1.LogPipelineInput{
					Application: &telemetryv1alpha1.LogPipelineApplicationInput{Enabled: ptr.To(true)},
					OTLP:        &telemetryv1alpha1.OTLPInput{Disabled: true},
				}
			},
			logProducerFunc: func(deploymentName, namespace string) client.Object {
				return stdloggen.NewDeployment(namespace).
					WithName(deploymentName).
					WithLabels(map[string]string{
						appLabelName: appLabelValue,
					}).
					K8sObject()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// t.Parallel()

			var (
				mockNs       = suite.IDWithSuffix(tc.name + "-mocks")
				genName      = suite.IDWithSuffix(tc.name + "-gen")
				pipelineName = suite.IDWithSuffix(tc.name + "-pipeline")
			)

			backend := backend.New(mockNs, backend.SignalTypeLogsOtel)

			hostSecretRef := backend.HostSecretRefV1Alpha1()
			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.logPipelineInputFunc(nil, nil)).
				WithOTLPOutput(
					testutils.OTLPEndpointFromSecret(
						hostSecretRef.Name,
						hostSecretRef.Namespace,
						hostSecretRef.Key,
					),
				).
				Build()

			resources := []client.Object{
				kitk8sobjects.NewNamespace(mockNs).K8sObject(),
				&pipeline,
				tc.logProducerFunc(genName, mockNs),
			}
			resources = append(resources, backend.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

			t.Log("Waiting for resources to be ready")

			assert.DeploymentReady(t.Context(), suite.K8sClient, kitkyma.LogGatewayName)
			assert.LogPipelineOtelHealthy(t.Context(), suite.K8sClient, pipelineName)
			assert.OtelLogsFromNamespaceDelivered(suite.ProxyClient, backend.ExportURL(suite.ProxyClient), mockNs)

			verifyServiceNameAttr(backend.ExportURL(suite.ProxyClient), genName, appLabelValue)
		})
	}
}

func skipIfDoesNotMatchLabel(t *testing.T, label string) {
	args := os.Args
	idx := slices.IndexFunc(args, func(arg string) bool {
		return strings.HasPrefix(arg, "label")
	})

	if idx == -1 {
		return
	}

	labelArg := args[idx]
	if parts := strings.Split(labelArg, "="); len(parts) == 2 {
		labelVal := parts[1]
		if labelVal != label {
			t.Skipf("Skipping test: label mismatch. Expected: %s, Got: %s", label, labelVal)
		}
	}
}

func verifyServiceNameAttr(backendExportURL, givenPodPrefix, expectedServiceName string) {
	Eventually(func(g Gomega) {
		resp, err := suite.ProxyClient.Get(backendExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		bodyContent, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(bodyContent).To(HaveFlatOtelLogs(
			ContainElement(SatisfyAll(
				HaveResourceAttributes(HaveKeyWithValue("service.name", expectedServiceName)),
				HaveResourceAttributes(HaveKeyWithValue("k8s.pod.name", ContainSubstring(givenPodPrefix))),
			)),
		))
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed(), fmt.Sprintf("could not find logs matching service.name: %s, k8s.pod.name: %s.*", expectedServiceName, givenPodPrefix))
}
