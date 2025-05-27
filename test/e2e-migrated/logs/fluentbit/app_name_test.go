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

func TestAppName(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()

		backendNs = uniquePrefix("backend")
	)

	nsNone := uniquePrefix("none")
	nsAppOnly := uniquePrefix("app-only")
	nsNameOnly := uniquePrefix("name-only")
	nsMixed := uniquePrefix("mixed")
	logProducerNone := stdloggen.NewDeployment(nsNone).WithName("none")
	logProducerAppOnly := stdloggen.NewDeployment(nsAppOnly).WithName("app-only").WithLabel("app", "app-only")
	logProducerNameOnly := stdloggen.NewDeployment(nsNameOnly).WithName("name-only").WithLabel("app.kubernetes.io/name", "name-only")
	logProducerMixed := stdloggen.NewDeployment(nsMixed).WithName("mixed").WithLabel("app", "app-mixed").WithLabel("app.kubernetes.io/name", "name-mixed")

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit)
	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithIncludeContainers(stdloggen.DefaultContainerName).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(nsNone).K8sObject(),
		kitk8s.NewNamespace(nsAppOnly).K8sObject(),
		kitk8s.NewNamespace(nsNameOnly).K8sObject(),
		kitk8s.NewNamespace(nsMixed).K8sObject(),
		logProducerNone.K8sObject(),
		logProducerAppOnly.K8sObject(),
		logProducerNameOnly.K8sObject(),
		logProducerMixed.K8sObject(),
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
	assert.DeploymentReady(t.Context(), logProducerNone.NamespacedName())
	assert.DeploymentReady(t.Context(), logProducerAppOnly.NamespacedName())
	assert.DeploymentReady(t.Context(), logProducerNameOnly.NamespacedName())
	assert.DeploymentReady(t.Context(), logProducerMixed.NamespacedName())

	assert.FluentBitLogsFromNamespaceDelivered(t.Context(), backend, nsNone)
	assert.FluentBitLogsFromNamespaceDelivered(t.Context(), backend, nsAppOnly)
	assert.FluentBitLogsFromNamespaceDelivered(t.Context(), backend, nsNameOnly)
	assert.FluentBitLogsFromNamespaceDelivered(t.Context(), backend, nsMixed)

	// No labels should not have app value
	assert.BackendDataEventuallyMatches(t.Context(), backend, fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
		fluentbit.HaveNamespace(Equal(nsNone)),
		fluentbit.HaveKubernetesAttributes(Not(HaveKey("app_name"))),
	))))

	// App only label should have app value of app label
	assert.BackendDataEventuallyMatches(t.Context(), backend, fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
		fluentbit.HaveNamespace(Equal(nsAppOnly)),
		fluentbit.HaveKubernetesAttributes(HaveKeyWithValue("app_name", "app-only"))),
	)))

	// Name only label should have app value of name label
	assert.BackendDataEventuallyMatches(t.Context(), backend, fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
		fluentbit.HaveNamespace(Equal(nsNameOnly)),
		fluentbit.HaveKubernetesAttributes(HaveKeyWithValue("app_name", "name-only"))),
	)))

	// Mixed label should have app value of name label
	assert.BackendDataEventuallyMatches(t.Context(), backend, fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
		fluentbit.HaveNamespace(Equal(nsMixed)),
		fluentbit.HaveKubernetesAttributes(HaveKeyWithValue("app_name", "name-mixed"))),
	)))
}
