package agent

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
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
		uniquePrefix      = unique.Prefix()
		pipelineName      = uniquePrefix()
		backendNs         = uniquePrefix("backend")
		genNs             = uniquePrefix("gen")
		levelINFOScenario = map[string]string{
			"scenario": "level-info",
			"level":    "INFO",
		}
		levelWarningScenario = map[string]string{
			"scenario": "level-warning",
			"level":    "WARNING",
		}
		logLevelScenario = map[string]string{
			"scenario":  "log.level",
			"log.level": "WARN",
		}
		defaultScenario = map[string]string{
			"scenario": "default",
		}
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)
	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithRuntimeInput(true,
			testutils.IncludeNamespaces(genNs)).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
		Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(genNs).K8sObject(),
		stdoutloggen.NewDeployment(genNs, stdoutloggen.WithFields(levelINFOScenario)).WithName(levelINFOScenario["scenario"]).K8sObject(),
		stdoutloggen.NewDeployment(genNs, stdoutloggen.WithFields(levelWarningScenario)).WithName(levelWarningScenario["scenario"]).K8sObject(),
		stdoutloggen.NewDeployment(genNs, stdoutloggen.WithFields(logLevelScenario)).WithName(logLevelScenario["scenario"]).K8sObject(),
		stdoutloggen.NewDeployment(genNs, stdoutloggen.WithFields(defaultScenario)).WithName(defaultScenario["scenario"]).K8sObject(),
		&pipeline,
	}
	resources = append(resources, backend.K8sObjects()...)

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
		assert.WithOptionalDescription("Scenario level-info should parse level attribute and remove it"),
	)

	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "level-warning")),
			HaveSeverityNumber(Equal(13)),
			HaveSeverityText(Equal("WARNING")),
			HaveAttributes(Not(HaveKey("level"))),
		))),
		assert.WithOptionalDescription("Scenario level-warning should parse level attribute and remove it"),
	)

	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "log.level")),
			HaveSeverityText(Equal("WARN")),
			HaveAttributes(Not(HaveKey("log.level"))),
		))),
		assert.WithOptionalDescription("Scenario log.level should parse log.level attribute and remove it"),
	)

	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "default")),
			HaveSeverityNumber(Equal(0)), // default value
			HaveSeverityText(BeEmpty()),
		))),
		assert.WithOptionalDescription("Default scenario should not have any severity"),
	)
}
