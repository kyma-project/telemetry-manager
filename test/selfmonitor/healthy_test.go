package selfmonitor

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	"github.com/kyma-project/telemetry-manager/test/testkit"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestHealthy(t *testing.T) {
	components := []string{
		suite.LabelLogAgent,
		suite.LabelLogGateway,
		suite.LabelFluentBit,
		suite.LabelMetricGateway,
		suite.LabelMetricAgent,
		suite.LabelTraces,
	}

	labels := []string{suite.LabelSelfMonitor, suite.LabelHealthy}
	labels = append(labels, components...)

	suite.SetupTestWithOptions(t, labels,
		kubeprep.WithGatewayReplicas(1),
		kubeprep.WithOverrideFIPSMode(false),
		kubeprep.WithFluentBitHostPathCleanup(),
	)
	enableDebugLogging(t)

	var (
		allResources []client.Object
		entries      []healthyEntry
	)

	for _, component := range components {
		pipelineName := fmt.Sprintf("selfmonitor-healthy-%s", component)
		uniquePrefix := unique.Prefix(component)
		backendNs := uniquePrefix("backend")
		genNs := uniquePrefix("gen")

		backend := kitbackend.New(backendNs, signalTypeForComponent(component))
		pipeline := buildPipeline(component, pipelineName, genNs, backend)

		allResources = append(allResources,
			kitk8sobjects.NewNamespace(backendNs).K8sObject(),
			kitk8sobjects.NewNamespace(genNs).K8sObject(),
			pipeline,
		)
		allResources = append(allResources, defaultGenerator(component)(genNs)...)
		allResources = append(allResources, backend.K8sObjects()...)

		entries = append(entries, healthyEntry{
			component:    component,
			backend:      backend,
			pipelineName: pipelineName,
			genNs:        genNs,
		})
	}

	Expect(kitk8s.CreateObjects(t, allResources...)).To(Succeed())

	for _, component := range components {
		logDiagnosticsOnFailure(t, component)
	}

	for _, e := range entries {
		assert.BackendReachable(t, e.backend)
	}

	assert.DeploymentReady(t, kitkyma.SelfMonitorName)
	assertSelfMonitorHasActiveTargets(t)

	FIPSModeEnabled, err := isFIPSModeEnabled(t)
	Expect(err).ToNot(HaveOccurred())

	if FIPSModeEnabled {
		assert.DeploymentHasImage(t, kitkyma.SelfMonitorName, names.SelfMonitorContainerName, testkit.SelfMonitorFIPSImage)
	} else {
		assert.DeploymentHasImage(t, kitkyma.SelfMonitorName, names.SelfMonitorContainerName, testkit.SelfMonitorImage)
	}

	// Wait for all components and pipelines to reach a healthy baseline.
	for _, e := range entries {
		assertComponentReady(t, e.component)
		assertPipelineHealthy(t, e.component, e.pipelineName)
	}

	// Wait for data to actually flow to each backend before starting the sustained check.
	for _, e := range entries {
		assertDataDeliveredEventually(t, e)
	}

	// Verify that data delivery and FlowHealthy conditions remain stable together for 3 minutes.
	assertAllHealthyConsistently(t, entries)
}

func isFIPSModeEnabled(t *testing.T) (bool, error) {
	const (
		managerContainerName = "manager"
		fipsEnvVarName       = "KYMA_FIPS_MODE_ENABLED"
	)

	var deployment appsv1.Deployment

	err := suite.K8sClient.Get(t.Context(), kitkyma.TelemetryManagerName, &deployment)
	if err != nil {
		return false, fmt.Errorf("failed to get manager deployment: %w", err)
	}

	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == managerContainerName {
			for _, env := range container.Env {
				if env.Name == fipsEnvVarName && env.Value == "true" {
					return true, nil
				}
			}

			return false, nil
		}
	}

	return false, fmt.Errorf("manager container not found in manager deployment")
}
