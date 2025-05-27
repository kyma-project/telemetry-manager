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

func TestBasePayloadWithHTTPOutput(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		genNs        = uniquePrefix("gen")
		backendNs    = uniquePrefix("backend")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit)
	logProducer := stdloggen.NewDeployment(genNs)
	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithIncludeContainers(stdloggen.DefaultContainerName).
		WithIncludeNamespaces(genNs).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		logProducer.K8sObject(),
		&pipeline,
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

	assert.FluentBitLogPipelineHealthy(t.Context(), pipelineName)
	assert.DaemonSetReady(t.Context(), kitkyma.FluentBitDaemonSetName)
	assert.DeploymentReady(t.Context(), backend.NamespacedName())
	assert.DeploymentReady(t.Context(), logProducer.NamespacedName())

	assert.BackendDataEventuallyMatches(
		t.Context(),
		backend,
		fluentbit.HaveFlatLogs(HaveEach(SatisfyAll(
			fluentbit.HaveAttributes(HaveKey("@timestamp")),
			fluentbit.HaveDateISO8601Format(BeTrue()),
		))),
		"Should have @timestamp and date attributes",
	)

	assert.BackendDataEventuallyMatches(
		t.Context(),
		backend,
		fluentbit.HaveFlatLogs(HaveEach(
			fluentbit.HaveKubernetesAttributes(SatisfyAll(
				HaveKey("pod_name"),
				HaveKey("pod_id"),
				HaveKey("pod_ip"),
				HaveKey("docker_id"),
				HaveKey("host"),
			)),
		)),
		"Should have typical Kubernetes attributes set by the kubernetes filter",
	)

	assert.BackendDataEventuallyMatches(
		t.Context(),
		backend,
		fluentbit.HaveFlatLogs(HaveEach(
			fluentbit.HaveKubernetesAttributes(SatisfyAll(
				HaveKeyWithValue("container_name", stdloggen.DefaultContainerName),
				HaveKeyWithValue("container_image", HaveSuffix(stdloggen.DefaultImageName)),
				HaveKeyWithValue("namespace_name", genNs),
			)),
		)),
		"Should have Kubernetes attributes with corresponding values set by the kubernetes filter",
	)

	assert.BackendDataEventuallyMatches(
		t.Context(),
		backend,
		fluentbit.HaveFlatLogs(HaveEach(
			fluentbit.HaveAttributes(HaveKey("cluster_identifier")),
		)),
		"Should have cluster identifier set by record_modifier filter",
	)

	assert.BackendDataEventuallyMatches(
		t.Context(),
		backend,
		fluentbit.HaveFlatLogs(HaveEach(
			fluentbit.HaveLogBody(Not(BeEmpty())),
		)),
		"Should have not-empty log body",
	)

	assert.BackendDataEventuallyMatches(
		t.Context(),
		backend,
		fluentbit.HaveFlatLogs(HaveEach(
			fluentbit.HaveAttributes(HaveKeyWithValue("stream", "stdout")),
		)),
		"Should have stream attribute",
	)
}
