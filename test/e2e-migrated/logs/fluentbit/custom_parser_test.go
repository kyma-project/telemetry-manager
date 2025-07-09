package fluentbit

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
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers/log/fluentbit"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestLogParser(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")

		parser = `Format regex
Regex  ^(?<user>[^ ]*) (?<pass>[^ ]*)$
Time_Key time
Time_Format %d/%b/%Y:%H:%M:%S %z
Types user:string pass:string`
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit)
	logProducer := stdloggen.NewDeployment(genNs).
		WithAnnotation("fluentbit.io/parser", "my-regex-parser")
	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
		Build()
	logRegexParser := kitk8s.NewLogParser("my-regex-parser", parser)

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		logProducer.K8sObject(),
		&pipeline,
		logRegexParser.K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

	assert.FluentBitLogPipelineHealthy(t, pipelineName)
	assert.DaemonSetReady(t.Context(), kitkyma.FluentBitDaemonSetName)
	assert.DeploymentReady(t.Context(), backend.NamespacedName())
	assert.DeploymentReady(t.Context(), logProducer.NamespacedName())

	assert.BackendDataEventuallyMatches(t, backend,
		fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
			fluentbit.HaveAttributes(HaveKeyWithValue("user", "foo")),
			fluentbit.HaveAttributes(HaveKeyWithValue("pass", "bar")),
		))),
	)
}
