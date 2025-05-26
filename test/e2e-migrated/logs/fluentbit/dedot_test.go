package fluentbit

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
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

func TestDedot(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		genNs        = uniquePrefix("gen")
		backendNs    = uniquePrefix("backend")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit)
	logProducer := stdloggen.NewDeployment(genNs).WithLabel("dedot.label", "logging-dedot-value")
	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithIncludeContainers(stdloggen.DefaultContainerName).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port()), testutils.HTTPDedot(true)).
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
	assert.DeploymentReady(t.Context(), types.NamespacedName{Name: stdloggen.DefaultName, Namespace: genNs})

	assert.BackendDataEventuallyMatches(t.Context(), backend, fluentbit.HaveFlatLogs(
		ContainElement(fluentbit.HaveKubernetesLabels(HaveKeyWithValue("dedot_label", "logging-dedot-value")))),
	)
}
