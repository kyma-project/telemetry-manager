package shared

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
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
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestServiceName_OTel(t *testing.T) {
	tests := []struct {
		label        string
		inputBuilder func(includeNs string) telemetryv1alpha1.LogPipelineInput
		expectAgent  bool
	}{
		{
			label: suite.LabelLogAgent,
			inputBuilder: func(includeNs string) telemetryv1alpha1.LogPipelineInput {
				return testutils.BuildLogPipelineApplicationInput(testutils.ExtIncludeNamespaces(includeNs))
			},
			expectAgent: true,
		},
		{
			label: suite.LabelLogGateway,
			inputBuilder: func(includeNs string) telemetryv1alpha1.LogPipelineInput {
				return testutils.BuildLogPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
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
				uniquePrefix    = unique.Prefix(tc.label)
				pipelineName    = uniquePrefix()
				deploymentName  = uniquePrefix()
				statefulSetName = uniquePrefix()
				backendNs       = uniquePrefix("backend")
				genNs           = uniquePrefix("gen")
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)
			hostSecretRef := backend.HostSecretRefV1Alpha1()

			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.inputBuilder(genNs)).
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
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(genNs).K8sObject(),
				&pipeline,
			}
			resources = append(resources, backend.K8sObjects()...)

			if tc.expectAgent {
				podSpecLogs := loggen.New(genNs).PodSpec()
				resources = append(resources,
					kitk8s.NewPod(podWithBothLabelsName, genNs).
						WithLabel(kubeAppLabelKey, kubeAppLabelValue).
						WithLabel(appLabelKey, appLabelValue).
						WithPodSpec(podSpecLogs).
						K8sObject(),
					kitk8s.NewJob(jobName, genNs).WithPodSpec(podSpecLogs).K8sObject(),
					kitk8s.NewPod(podWithNoLabelsName, genNs).WithPodSpec(podSpecLogs).K8sObject(),
				)
			} else {
				podSpecWithUndefinedService := telemetrygen.PodSpec(telemetrygen.SignalTypeLogs, telemetrygen.WithServiceName(""))
				resources = append(resources,
					kitk8s.NewPod(podWithAppLabelName, genNs).
						WithLabel(appLabelKey, appLabelValue).
						WithPodSpec(podSpecWithUndefinedService).
						K8sObject(),
					kitk8s.NewDeployment(deploymentName, genNs).WithPodSpec(podSpecWithUndefinedService).K8sObject(),
					kitk8s.NewStatefulSet(statefulSetName, genNs).WithPodSpec(podSpecWithUndefinedService).K8sObject(),
				)
			}

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
			})
			Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

			assert.DeploymentReady(t.Context(), suite.K8sClient, kitkyma.LogGatewayName)

			if tc.expectAgent {
				assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.LogAgentName)
			}

			assert.DeploymentReady(t.Context(), suite.K8sClient, kitkyma.LogGatewayName)
			assert.DeploymentReady(t.Context(), suite.K8sClient, backend.NamespacedName())
			assert.OTelLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineName)
			assert.OTelLogsFromNamespaceDelivered(t.Context(), backend, genNs)

			verifyServiceNameAttr := func(givenPodPrefix, expectedServiceName string) {
				assert.BackendDataEventuallyMatching(t.Context(), backend, HaveFlatOTelLogs(
					ContainElement(SatisfyAll(
						HaveResourceAttributes(HaveKeyWithValue(serviceKey, expectedServiceName)),
						HaveResourceAttributes(HaveKeyWithValue(podKey, ContainSubstring(givenPodPrefix))),
					)),
				))
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
			assert.BackendDataConsistentlyMatching(t.Context(), backend, HaveFlatOTelLogs(
				Not(ContainElement(
					HaveResourceAttributes(HaveKey(ContainSubstring("kyma"))),
				)),
			))
		})
	}
}
