package shared

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"

	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TODO: TO BE FIXED
func TestServiceName_OTel(t *testing.T) {
	tests := []struct {
		label string
		input telemetryv1alpha1.LogPipelineInput
	}{
		{
			label: suite.LabelLogAgent,
			input: testutils.BuildLogPipelineApplicationInput(),
		},
		{
			label: suite.LabelLogGateway,
			input: testutils.BuildLogPipelineOTLPInput(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label)

			const (
				jobName               = "job"
				podWithNoLabelsName   = "pod-with-no-labels"
				kubeAppLabelKey       = "app.kubernetes.io/name"
				kubeAppLabelValue     = "kube-workload"
				appLabelKey           = "app"
				appLabelValue         = "workload"
				podWithBothLabelsName = "pod-with-both-app-labels" // #nosec G101 -- This is a false positive
			)

			var (
				uniquePrefix = unique.Prefix()
				pipelineName = uniquePrefix()
				mockNs       = uniquePrefix()
			)

			backend := kitbackend.New(mockNs, kitbackend.SignalTypeLogsOTel)
			backendExportURL := backend.ExportURL(suite.ProxyClient)
			hostSecretRef := backend.HostSecretRefV1Alpha1()

			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.input).
				WithOTLPOutput(
					testutils.OTLPEndpointFromSecret(
						hostSecretRef.Name,
						hostSecretRef.Namespace,
						hostSecretRef.Key,
					),
				).
				Build()

			logGenerator := loggen.New(mockNs)

			resources := []client.Object{
				kitk8s.NewNamespace(mockNs).K8sObject(),
				&pipeline,
				kitk8s.NewPod(podWithBothLabelsName, mockNs).
					WithLabel(kubeAppLabelKey, kubeAppLabelValue).
					WithLabel(appLabelKey, appLabelValue).
					WithPodSpec(logGenerator.PodSpec()).
					K8sObject(),
				kitk8s.NewJob(jobName, mockNs).WithPodSpec(logGenerator.PodSpec()).K8sObject(),
				kitk8s.NewPod(podWithNoLabelsName, mockNs).WithPodSpec(logGenerator.PodSpec()).K8sObject(),
			}
			resources = append(resources, backend.K8sObjects()...)

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
			})
			Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

			assert.DeploymentReady(suite.Ctx, suite.K8sClient, kitkyma.LogGatewayName)
			assert.DaemonSetReady(suite.Ctx, suite.K8sClient, kitkyma.LogAgentName)
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: kitbackend.DefaultName, Namespace: mockNs})
			assert.OTelLogPipelineHealthy(suite.Ctx, suite.K8sClient, pipelineName)
			assert.OTelLogsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, mockNs)

			verifyServiceNameAttr := func(givenPodPrefix, expectedServiceName string) {
				Eventually(func(g Gomega) {
					resp, err := suite.ProxyClient.Get(backendExportURL)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

					bodyContent, err := io.ReadAll(resp.Body)
					defer resp.Body.Close()
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(bodyContent).To(HaveFlatOTelLogs(
						ContainElement(SatisfyAll(
							HaveResourceAttributes(HaveKeyWithValue("service.name", expectedServiceName)),
							HaveResourceAttributes(HaveKeyWithValue("k8s.pod.name", ContainSubstring(givenPodPrefix))),
						)),
					))
				}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed(), fmt.Sprintf("could not find logs matching service.name: %s, k8s.pod.name: %s.*", expectedServiceName, givenPodPrefix))
			}

			verifyServiceNameAttr(podWithBothLabelsName, kubeAppLabelValue)
			verifyServiceNameAttr(jobName, jobName)
			verifyServiceNameAttr(podWithNoLabelsName, podWithNoLabelsName)
			assert.TelemetryDataDelivered(suite.ProxyClient, backendExportURL, HaveFlatOTelLogs(
				Not(ContainElement(
					HaveResourceAttributes(HaveKey(ContainSubstring("kyma"))),
				)),
			))
		})
	}
}
