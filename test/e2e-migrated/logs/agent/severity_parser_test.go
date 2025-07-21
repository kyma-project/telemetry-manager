package agent

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestSeverityParser(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelLogAgent)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
	)

	levelINFOScenario := map[string]string{
		"scenario": "level-info",
		"level":    "INFO",
	}
	levelWarningScenario := map[string]string{
		"scenario": "level-warning",
		"level":    "warning",
	}
	logLevelScenario := map[string]string{
		"scenario":  "log.level",
		"log.level": "WARN",
	}
	noLevelScenario := map[string]string{
		"scenario": "no-level",
	}

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)
	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithApplicationInput(true,
			[]testutils.ExtendedNamespaceSelectorOptions{testutils.ExtIncludeNamespaces(genNs)}...).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		stdoutloggen.NewDeploymentWithName(levelINFOScenario["scenario"], genNs, stdoutloggen.WithFields(levelINFOScenario)).K8sObject(),
		stdoutloggen.NewDeploymentWithName(levelWarningScenario["scenario"], genNs, stdoutloggen.WithFields(levelWarningScenario)).K8sObject(),
		stdoutloggen.NewDeploymentWithName(logLevelScenario["scenario"], genNs, stdoutloggen.WithFields(logLevelScenario)).K8sObject(),
		stdoutloggen.NewDeploymentWithName(noLevelScenario["scenario"], genNs, stdoutloggen.WithFields(noLevelScenario)).K8sObject(),
		&pipeline,
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DeploymentReady(t, kitkyma.LogGatewayName)
	assert.DaemonSetReady(t, kitkyma.LogAgentName)
	assert.OTelLogPipelineHealthy(t, pipelineName)

	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "level-info")),
			HaveSeverityNumber(Equal(9)),
			HaveSeverityText(Equal("INFO")),
			HaveAttributes(Not(HaveKey("level"))),
		))),
		"Scenario level-info should parse level attribute and remove it",
	)

	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "level-warning")),
			HaveSeverityNumber(Equal(13)),
			HaveSeverityText(Equal("warning")),
			HaveAttributes(Not(HaveKey("level"))),
		))),
		"Scenario level-warning should parse level attribute and remove it",
	)

	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "log.level")),
			HaveSeverityText(Equal("WARN")),
			HaveAttributes(Not(HaveKey("log.level"))),
		))),
		"Scenario log.level should parse log.level attribute and remove it",
	)

	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "no-level")),
			HaveSeverityNumber(Equal(0)), // default value
			HaveSeverityText(BeEmpty()),
		))),
		"Default scenario should not have any severity",
	)
}
