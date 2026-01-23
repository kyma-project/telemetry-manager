package shared

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
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

func TestMultiPipelineBroken_OTel(t *testing.T) {
	tests := []struct {
		name                string
		labels              []string
		inputBuilder        func(includeNs string) telemetryv1beta1.LogPipelineInput
		logGeneratorBuilder func(ns string) client.Object
		resourceName        types.NamespacedName
		readinessCheckFunc  func(t *testing.T, name types.NamespacedName)
	}{
		{
			name:   suite.LabelLogAgent,
			labels: []string{suite.LabelLogAgent},
			inputBuilder: func(includeNs string) telemetryv1beta1.LogPipelineInput {
				return testutils.BuildLogPipelineRuntimeInput(testutils.IncludeNamespaces(includeNs))
			},
			logGeneratorBuilder: func(ns string) client.Object {
				return stdoutloggen.NewDeployment(ns).K8sObject()
			},
			resourceName:       kitkyma.LogAgentName,
			readinessCheckFunc: assert.DaemonSetReady,
		},
		{
			name:   suite.LabelLogGateway,
			labels: []string{suite.LabelLogGateway},
			inputBuilder: func(includeNs string) telemetryv1beta1.LogPipelineInput {
				return testutils.BuildLogPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
			logGeneratorBuilder: func(ns string) client.Object {
				return telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeLogs).K8sObject()
			},
			resourceName:       kitkyma.LogGatewayName,
			readinessCheckFunc: assert.DeploymentReady,
		},
		{
			name:   fmt.Sprintf("%s-%s", suite.LabelLogGateway, suite.LabelExperimental),
			labels: []string{suite.LabelLogGateway, suite.LabelExperimental},
			inputBuilder: func(includeNs string) telemetryv1beta1.LogPipelineInput {
				return testutils.BuildLogPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
			logGeneratorBuilder: func(ns string) client.Object {
				return telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeCentralLogs).K8sObject()
			},
			resourceName:       kitkyma.TelemetryOTLPGatewayName,
			readinessCheckFunc: assert.DaemonSetReady,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.labels...)

			var (
				uniquePrefix        = unique.Prefix(tc.name)
				healthyPipelineName = uniquePrefix("healthy")
				brokenPipelineName  = uniquePrefix("broken")
				backendNs           = uniquePrefix("backend")
				genNs               = uniquePrefix("gen")
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)

			healthyPipeline := testutils.NewLogPipelineBuilder().
				WithName(healthyPipelineName).
				WithInput(tc.inputBuilder(genNs)).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
				Build()

			brokenPipeline := testutils.NewLogPipelineBuilder().
				WithName(brokenPipelineName).
				WithInput(tc.inputBuilder(genNs)).
				WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("dummy", "dummy", "user", "password")). // broken pipeline references a secret that does not exist
				Build()

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				&healthyPipeline,
				&brokenPipeline,
				tc.logGeneratorBuilder(genNs),
			}
			resources = append(resources, backend.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backend)

			tc.readinessCheckFunc(t, tc.resourceName)

			assert.OTelLogPipelineHealthy(t, healthyPipeline.Name)
			assert.LogPipelineHasCondition(t, brokenPipeline.Name, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonReferencedSecretMissing,
			})

			assert.OTelLogsFromNamespaceDelivered(t, backend, genNs)
		})
	}
}

func TestMultiPipelineBroken_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix        = unique.Prefix()
		healthyPipelineName = uniquePrefix("healthy")
		brokenPipelineName  = uniquePrefix("broken")
		backendNs           = uniquePrefix("backend")
		genNs               = uniquePrefix("gen")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit)

	healthyPipeline := testutils.NewLogPipelineBuilder().
		WithName(healthyPipelineName).
		WithRuntimeInput(true).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
		Build()

	brokenPipeline := testutils.NewLogPipelineBuilder().
		WithName(brokenPipelineName).
		WithRuntimeInput(true).
		WithHTTPOutput(testutils.HTTPHostFromSecret("dummy", "dummy", "dummy")). // broken pipeline ref
		Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(genNs).K8sObject(),
		&healthyPipeline,
		&brokenPipeline,
		stdoutloggen.NewDeployment(genNs).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
	assert.FluentBitLogPipelineHealthy(t, healthyPipeline.Name)

	assert.LogPipelineHasCondition(t, brokenPipeline.Name, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonReferencedSecretMissing,
	})
	assert.FluentBitLogsFromNamespaceDelivered(t, backend, genNs)
}
