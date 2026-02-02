package shared

import (
	"fmt"
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
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestServiceName_OTel(t *testing.T) {
	tests := []struct {
		name               string
		labels             []string
		inputBuilder       func(includeNs string) telemetryv1beta1.LogPipelineInput
		expectAgent        bool
		resourceName       types.NamespacedName
		readinessCheckFunc func(t *testing.T, name types.NamespacedName)
		genSignalType      telemetrygen.SignalType
	}{
		{
			name:   suite.LabelLogAgent,
			labels: []string{suite.LabelLogAgent},
			inputBuilder: func(includeNs string) telemetryv1beta1.LogPipelineInput {
				return testutils.BuildLogPipelineRuntimeInput(testutils.IncludeNamespaces(includeNs))
			},
			expectAgent:        true,
			resourceName:       kitkyma.LogAgentName,
			readinessCheckFunc: assert.DaemonSetReady,
		},
		{
			name:   suite.LabelLogGateway,
			labels: []string{suite.LabelLogGateway},
			inputBuilder: func(includeNs string) telemetryv1beta1.LogPipelineInput {
				return testutils.BuildLogPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
			expectAgent:        false,
			resourceName:       kitkyma.LogGatewayName,
			readinessCheckFunc: assert.DeploymentReady,
			genSignalType:      telemetrygen.SignalTypeLogs,
		},
		{
			name:   fmt.Sprintf("%s-%s", suite.LabelLogGateway, suite.LabelExperimental),
			labels: []string{suite.LabelLogGateway, suite.LabelExperimental},
			inputBuilder: func(includeNs string) telemetryv1beta1.LogPipelineInput {
				return testutils.BuildLogPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
			expectAgent:        false,
			resourceName:       kitkyma.TelemetryOTLPGatewayName,
			readinessCheckFunc: assert.DaemonSetReady,
			genSignalType:      telemetrygen.SignalTypeCentralLogs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.labels...)

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
				uniquePrefix    = unique.Prefix(tc.name)
				pipelineName    = uniquePrefix()
				deploymentName  = uniquePrefix()
				statefulSetName = uniquePrefix()
				backendNs       = uniquePrefix("backend")
				genNs           = uniquePrefix("gen")
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)
			hostSecretRef := backend.HostSecretRefV1Beta1()

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
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				&pipeline,
			}
			resources = append(resources, backend.K8sObjects()...)

			if tc.expectAgent {
				podSpecLogs := stdoutloggen.PodSpec()
				resources = append(resources,
					kitk8sobjects.NewPod(podWithBothLabelsName, genNs).
						WithLabel(kubeAppLabelKey, kubeAppLabelValue).
						WithLabel(appLabelKey, appLabelValue).
						WithPodSpec(podSpecLogs).
						K8sObject(),
					kitk8sobjects.NewJob(jobName, genNs).WithPodSpec(podSpecLogs).K8sObject(),
					kitk8sobjects.NewPod(podWithNoLabelsName, genNs).WithPodSpec(podSpecLogs).K8sObject(),
				)
			} else {
				podSpecWithUndefinedService := telemetrygen.PodSpec(tc.genSignalType, telemetrygen.WithServiceName(""))
				resources = append(resources,
					kitk8sobjects.NewPod(podWithAppLabelName, genNs).
						WithLabel(appLabelKey, appLabelValue).
						WithPodSpec(podSpecWithUndefinedService).
						K8sObject(),
					kitk8sobjects.NewDeployment(deploymentName, genNs).WithPodSpec(podSpecWithUndefinedService).K8sObject(),
					kitk8sobjects.NewStatefulSet(statefulSetName, genNs).WithPodSpec(podSpecWithUndefinedService).K8sObject(),
				)
			}

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			tc.readinessCheckFunc(t, tc.resourceName)

			assert.BackendReachable(t, backend)
			assert.OTelLogPipelineHealthy(t, pipelineName)
			assert.OTelLogsFromNamespaceDelivered(t, backend, genNs)

			verifyServiceNameAttr := func(givenPodPrefix, expectedServiceName string) {
				assert.BackendDataEventuallyMatches(t, backend,
					HaveFlatLogs(ContainElement(SatisfyAll(
						HaveResourceAttributes(HaveKeyWithValue(serviceKey, expectedServiceName)),
						HaveResourceAttributes(HaveKeyWithValue(podKey, ContainSubstring(givenPodPrefix))),
					))),
				)
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
			assert.BackendDataConsistentlyMatches(t, backend,
				HaveFlatLogs(Not(ContainElement(
					HaveResourceAttributes(HaveKey(ContainSubstring("kyma"))),
				))),
			)
		})
	}
}
