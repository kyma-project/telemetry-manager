package fluentbit

import (
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

func TestBasePayloadWithHTTPOutput(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit)
	logProducer := stdoutloggen.NewDeployment(genNs)
	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithIncludeContainers(stdoutloggen.DefaultContainerName).
		WithIncludeNamespaces(genNs).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
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
	assert.DeploymentReady(t, logProducer.NamespacedName())
	assert.FluentBitLogPipelineHealthy(t, pipelineName)

	assert.BackendDataEventuallyMatches(t, backend,
		fluentbit.HaveFlatLogs(HaveEach(SatisfyAll(
			fluentbit.HaveAttributes(HaveKey("@timestamp")),
			fluentbit.HaveDateISO8601Format(BeTrue()),
		))),
		assert.WithOptionalDescription("Should have @timestamp and date attributes"),
	)

	assert.BackendDataEventuallyMatches(t, backend,
		fluentbit.HaveFlatLogs(HaveEach(
			fluentbit.HaveKubernetesAttributes(SatisfyAll(
				HaveKey("pod_name"),
				HaveKey("pod_id"),
				HaveKey("docker_id"),
				HaveKey("host"),
			)),
		)),
		assert.WithOptionalDescription("Should have typical Kubernetes attributes set by the kubernetes filter"),
	)

	assert.BackendDataEventuallyMatches(t, backend,
		fluentbit.HaveFlatLogs(HaveEach(
			fluentbit.HaveKubernetesAttributes(SatisfyAll(
				HaveKeyWithValue("container_name", stdoutloggen.DefaultContainerName),
				HaveKeyWithValue("container_image", HaveSuffix(stdoutloggen.DefaultImageName)),
				HaveKeyWithValue("namespace_name", genNs),
			)),
		)),
		assert.WithOptionalDescription("Should have Kubernetes attributes with corresponding values set by the kubernetes filter"),
	)

	assert.BackendDataEventuallyMatches(t, backend,
		fluentbit.HaveFlatLogs(HaveEach(
			fluentbit.HaveAttributes(HaveKey("cluster_identifier")),
		)),
		assert.WithOptionalDescription("Should have cluster identifier set by record_modifier filter"),
	)

	assert.BackendDataEventuallyMatches(t, backend,
		fluentbit.HaveFlatLogs(HaveEach(
			fluentbit.HaveLogBody(Not(BeEmpty())),
		)),
		assert.WithOptionalDescription("Should have not-empty log body"),
	)

	assert.BackendDataEventuallyMatches(t, backend,
		fluentbit.HaveFlatLogs(HaveEach(
			fluentbit.HaveAttributes(HaveKeyWithValue("stream", "stdout")),
		)),
		assert.WithOptionalDescription("Should have stream attribute"),
	)
}
