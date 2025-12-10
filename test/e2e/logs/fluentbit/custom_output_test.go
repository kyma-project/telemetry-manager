package fluentbit

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers/log/fluentbit"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestCustomOutput(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit)
	logProducer := stdoutloggen.NewDeployment(genNs)
	customOutputTemplate := fmt.Sprintf(`
	name   http
	port   %d
	host   %s
	format json`, backend.Port(), backend.Host())
	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithCustomOutput(customOutputTemplate).
		Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(genNs).K8sObject(),
		logProducer.K8sObject(),
		&pipeline,
	}
	resources = append(resources, backend.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
	assert.FluentBitLogPipelineHealthy(t, pipelineName)

	assert.LogPipelineUnsupportedMode(t, pipelineName, true)
	assert.FluentBitLogsFromPodDelivered(t, backend, stdoutloggen.DefaultName)

	assert.BackendDataEventuallyMatches(t, backend,
		fluentbit.HaveFlatLogs(HaveEach(SatisfyAll(
			fluentbit.HaveAttributes(HaveKey("cluster_identifier")),
			fluentbit.HaveAttributes(Not(HaveKey("@timestamp"))),
			fluentbit.HaveKubernetesAttributes(Not(HaveKey("app_name"))),
			fluentbit.HaveLogBody(Not(BeEmpty())),
			fluentbit.HaveAttributes(HaveKey("stream")),
		))),
	)
}
