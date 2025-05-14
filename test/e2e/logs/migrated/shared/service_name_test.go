package shared

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestServiceName_OTel(t *testing.T) {
	tests := []struct {
		label       string
		input       telemetryv1alpha1.LogPipelineInput
		expectAgent bool
	}{
		{
			label:       suite.LabelLogAgent,
			input:       testutils.BuildLogPipelineApplicationInput(),
			expectAgent: true,
		},
		{
			label:       suite.LabelLogGateway,
			input:       testutils.BuildLogPipelineOTLPInput(),
			expectAgent: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label)

			const (
				jobName               = "job"
				podWithNoLabelsName   = "pod-with-no-labels"
				podWithAppLabelName   = "pod-with-app-label"
				kubeAppLabelKey       = "app.kubernetes.io/name"
				kubeAppLabelValue     = "kube-workload"
				appLabelKey           = "app"
				appLabelValue         = "workload"
				podWithBothLabelsName = "pod-with-both-app-labels" // #nosec G101 -- This is a false positive
				serviceKey            = "service.name"
				podKey                = "k8s.pod.name"
			)

			var (
				uniquePrefix    = unique.Prefix()
				pipelineName    = uniquePrefix()
				deploymentName  = uniquePrefix("gateway")
				statefulSetName = uniquePrefix("gateway")
				mockNs          = uniquePrefix()
			)

			backend := kitbackend.New(mockNs, kitbackend.SignalTypeLogsOTel)
			backendExportURL := backend.ExportURL(suite.ProxyClient)
			hostSecretRef := backend.HostSecretRefV1Alpha1()

			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.input).
				WithKeepOriginalBody(tc.expectAgent).
				WithOTLPOutput(
					testutils.OTLPEndpointFromSecret(
						hostSecretRef.Name,
						hostSecretRef.Namespace,
						hostSecretRef.Key,
					),
				).
				Build()

			resources := []client.Object{
				kitk8s.NewNamespace(mockNs).K8sObject(),
				&pipeline,
			}
			resources = append(resources, backend.K8sObjects()...)

			if tc.expectAgent {
				podSpecLogs := loggen.New(mockNs).PodSpec()
				resources = append(resources,
					kitk8s.NewPod(podWithBothLabelsName, mockNs).
						WithLabel(kubeAppLabelKey, kubeAppLabelValue).
						WithLabel(appLabelKey, appLabelValue).
						WithPodSpec(podSpecLogs).
						K8sObject(),
					kitk8s.NewJob(jobName, mockNs).WithPodSpec(podSpecLogs).K8sObject(),
					kitk8s.NewPod(podWithNoLabelsName, mockNs).WithPodSpec(podSpecLogs).K8sObject(),
				)
			} else {
				podSpecWithUndefinedService := telemetrygen.PodSpec(telemetrygen.SignalTypeLogs, telemetrygen.WithServiceName(""))
				resources = append(resources,
					kitk8s.NewPod(podWithAppLabelName, mockNs).
						WithLabel(appLabelKey, appLabelValue).
						WithPodSpec(podSpecWithUndefinedService).
						K8sObject(),
					kitk8s.NewDeployment(deploymentName, mockNs).WithPodSpec(podSpecWithUndefinedService).K8sObject(),
					kitk8s.NewStatefulSet(statefulSetName, mockNs).WithPodSpec(podSpecWithUndefinedService).K8sObject(),
				)
			}

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
			})
			Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

			assert.DeploymentReady(suite.Ctx, suite.K8sClient, kitkyma.LogGatewayName)

			if tc.expectAgent {
				assert.DaemonSetReady(suite.Ctx, suite.K8sClient, kitkyma.LogAgentName)
			}

			Eventually(func(g Gomega) int32 {
				var deployment appsv1.Deployment
				err := suite.K8sClient.Get(suite.Ctx, kitkyma.LogGatewayName, &deployment)
				g.Expect(err).NotTo(HaveOccurred())
				return *deployment.Spec.Replicas
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(int32(2)))
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
							HaveResourceAttributes(HaveKeyWithValue(serviceKey, expectedServiceName)),
							HaveResourceAttributes(HaveKeyWithValue(podKey, ContainSubstring(givenPodPrefix))),
						)),
					))
				}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed(), fmt.Sprintf("could not find logs matching service.name: %s, k8s.pod.name: %s.*", expectedServiceName, givenPodPrefix))
			}

			if tc.expectAgent {
				verifyServiceNameAttr(podWithBothLabelsName, kubeAppLabelValue)
				verifyServiceNameAttr(jobName, jobName)
				verifyServiceNameAttr(podWithNoLabelsName, podWithNoLabelsName)
			} else {
				verifyServiceNameAttr(podWithAppLabelName, appLabelValue)
				verifyServiceNameAttr(deploymentName, deploymentName)
				verifyServiceNameAttr(statefulSetName, statefulSetName)
			}

			// Verify that temporary kyma resource attributes are removed from the logs
			assert.DataEventuallyMatching(suite.ProxyClient, backendExportURL, HaveFlatOTelLogs(
				Not(ContainElement(
					HaveResourceAttributes(HaveKey(ContainSubstring("kyma"))),
				)),
			))
		})
	}
}
