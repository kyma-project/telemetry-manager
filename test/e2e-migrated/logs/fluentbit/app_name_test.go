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
	logProducerNone := loggen.New(nsNone).WithName("none").WithUseJSON().WithLabels(map[string]string{})
	logProducerAppOnly := loggen.New(nsAppOnly).WithName("app-only").WithUseJSON().WithLabels(map[string]string{"app": "app-only"})
	logProducerNameOnly := loggen.New(nsNameOnly).WithName("name-only").WithUseJSON().WithLabels(map[string]string{"app.kubernetes.io/name": "name-only"})
	logProducerMixed := loggen.New(nsMixed).WithName("mixed").WithUseJSON().WithLabels(map[string]string{"app": "app-mixed", "app.kubernetes.io/name": "name-mixed"})

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit)
	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithIncludeContainers(loggen.DefaultContainerName).
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
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineName)
	assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.FluentBitDaemonSetName)
	assert.DeploymentReady(t.Context(), suite.K8sClient, backend.NamespacedName())
	assert.DeploymentReady(t.Context(), suite.K8sClient, logProducerNone.NamespacedName())
	assert.DeploymentReady(t.Context(), suite.K8sClient, logProducerAppOnly.NamespacedName())
	assert.DeploymentReady(t.Context(), suite.K8sClient, logProducerNameOnly.NamespacedName())
	assert.DeploymentReady(t.Context(), suite.K8sClient, logProducerMixed.NamespacedName())

	assert.FluentBitLogsFromNamespaceDelivered(t.Context(), backend, nsNone)
	assert.FluentBitLogsFromNamespaceDelivered(t.Context(), backend, nsAppOnly)
	assert.FluentBitLogsFromNamespaceDelivered(t.Context(), backend, nsNameOnly)
	assert.FluentBitLogsFromNamespaceDelivered(t.Context(), backend, nsMixed)

	// No labels should not have app value
	assert.BackendDataEventuallyMatches(t.Context(), backend, fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
		fluentbit.HaveNamespace(Equal(nsNone)),
		fluentbit.HaveAttributes(Not(HaveKey("app_name"))),
	))))

	// App only label should have app value of app label
	assert.BackendDataEventuallyMatches(t.Context(), backend, fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
		fluentbit.HaveNamespace(Equal(nsAppOnly)),
		fluentbit.HaveAttributes(HaveKeyWithValue("app_name", "app-only"))),
	)))

	// Name only label should have app value of name label
	assert.BackendDataEventuallyMatches(t.Context(), backend, fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
		fluentbit.HaveNamespace(Equal(nsAppOnly)),
		fluentbit.HaveAttributes(HaveKeyWithValue("app_name", "name-only"))),
	)))

	// Mixed label should have app value of name label
	assert.BackendDataEventuallyMatches(t.Context(), backend, fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
		fluentbit.HaveNamespace(Equal(nsAppOnly)),
		fluentbit.HaveAttributes(HaveKeyWithValue("app_name", "name-mixed"))),
	)))
}
