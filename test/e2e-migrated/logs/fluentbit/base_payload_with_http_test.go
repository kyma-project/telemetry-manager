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
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestBasePayloadWithHttpOutput(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		genNs        = uniquePrefix("gen")
		backendNs    = uniquePrefix("backend")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit)
	logProducer := loggen.New(genNs).WithUseJSON()
	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithIncludeContainers(loggen.DefaultContainerName).
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
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineName)
	assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.FluentBitDaemonSetName)
	assert.DeploymentReady(t.Context(), suite.K8sClient, backend.NamespacedName())
	assert.DeploymentReady(t.Context(), suite.K8sClient, logProducer.NamespacedName())

	assert.BackendDataEventuallyMatches(t.Context(), backend, fluentbit.HaveFlatLogs(HaveEach(SatisfyAll(
		// timestamps
		fluentbit.HaveAttributes(HaveKey("@timestamp")),
		fluentbit.HaveDateISO8601Format(BeTrue()),

		// kubernetes filter
		fluentbit.HaveKubernetesAttributes(HaveKey("container_hash")),
		fluentbit.HaveKubernetesAttributes(HaveKeyWithValue("container_name", loggen.DefaultContainerName)),
		fluentbit.HaveKubernetesAttributes(HaveKeyWithValue("container_image", loggen.DefaultImageName)),
		fluentbit.HaveKubernetesAttributes(HaveKeyWithValue("namespace_name", genNs)),
		fluentbit.HaveKubernetesAttributes(HaveKey("pod_name")),
		fluentbit.HaveKubernetesAttributes(HaveKey("pod_id")),
		fluentbit.HaveKubernetesAttributes(HaveKey("pod_ip")),
		fluentbit.HaveKubernetesAttributes(HaveKey("docker_id")),
		fluentbit.HaveKubernetesAttributes(HaveKey("host")),
		fluentbit.HaveKubernetesLabels(HaveKeyWithValue("app", loggen.DefaultName)),

		// record_modifier filter
		fluentbit.HaveAttributes(HaveKey("cluster_identifier")),

		// base attributes
		fluentbit.HaveLogBody(Not(BeEmpty())),
		fluentbit.HaveAttributes(HaveKeyWithValue("stream", "stdout")),
	))))
}
