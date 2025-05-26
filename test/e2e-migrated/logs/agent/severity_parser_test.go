package agent

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestSeverityParser(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelLogAgent)

	var (
		uniquePrefix = unique.Prefix()
		genNs        = uniquePrefix("generator")
		backendNs    = uniquePrefix("backend")
		pipelineName = uniquePrefix()
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)
	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithApplicationInput(true,
			[]testutils.ExtendedNamespaceSelectorOptions{testutils.ExtIncludeNamespaces(genNs)}...).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()

	var resources []client.Object
	resources = append(resources,
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		stdloggen.NewDeployment(
			genNs,
			stdloggen.AppendLogLines(
				`{"scenario": "levelAndINFO", "level": "INFO"}`,
				`{"scenario": "levelAndWarning", "level": "warning"}`,
				`{"scenario": "log.level", "log.level":"WARN"}`,
				`{"scenario": "noLevel"}`,
			),
		).K8sObject(),
		&pipeline,
	)
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

	assert.DeploymentReady(t.Context(), kitkyma.LogGatewayName)
	assert.DeploymentReady(t.Context(), backend.NamespacedName())
	assert.DaemonSetReady(t.Context(), kitkyma.LogAgentName)

	assert.OTelLogPipelineHealthy(t.Context(), pipelineName)

	t.Log("Scenario levelAndINFO should parse level attribute and remove it")
	assert.BackendDataEventuallyMatches(t.Context(), backend, HaveFlatLogs(
		ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "levelAndINFO")),
			HaveSeverityNumber(Equal(9)),
			HaveSeverityText(Equal("INFO")),
			HaveAttributes(Not(HaveKey("level"))),
		)),
	))

	t.Log("Scenario levelAndWarning should parse level attribute and remove it")
	assert.BackendDataEventuallyMatches(t.Context(), backend, HaveFlatLogs(
		ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "levelAndWarning")),
			HaveSeverityNumber(Equal(13)),
			HaveSeverityText(Equal("warning")),
			HaveAttributes(Not(HaveKey("level"))),
		)),
	))

	t.Log("Scenario log.level should parse log.level attribute and remove it")
	assert.BackendDataEventuallyMatches(t.Context(), backend, HaveFlatLogs(
		ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "log.level")),
			HaveSeverityText(Equal("WARN")),
			HaveAttributes(Not(HaveKey("log.level"))),
		)),
	))

	t.Log("Default scenario should not have any severity")
	assert.BackendDataEventuallyMatches(t.Context(), backend, HaveFlatLogs(
		ContainElement(SatisfyAll(
			HaveLogBody(Equal(stdloggen.DefaultLine)),
			HaveSeverityNumber(Equal(0)), // default value
			HaveSeverityText(BeEmpty()),
		)),
	))
}
