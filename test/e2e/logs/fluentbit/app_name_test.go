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
	logProducerNone := stdoutloggen.NewDeployment(nsNone).WithName("none")
	logProducerAppOnly := stdoutloggen.NewDeployment(nsAppOnly).WithName("app-only").WithLabel("app", "app-only")
	logProducerNameOnly := stdoutloggen.NewDeployment(nsNameOnly).WithName("name-only").WithLabel("app.kubernetes.io/name", "name-only")
	logProducerMixed := stdoutloggen.NewDeployment(nsMixed).WithName("mixed").WithLabel("app", "app-mixed").WithLabel("app.kubernetes.io/name", "name-mixed")

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit)
	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithIncludeContainers(stdoutloggen.DefaultContainerName).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
		Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(nsNone).K8sObject(),
		kitk8sobjects.NewNamespace(nsAppOnly).K8sObject(),
		kitk8sobjects.NewNamespace(nsNameOnly).K8sObject(),
		kitk8sobjects.NewNamespace(nsMixed).K8sObject(),
		logProducerNone.K8sObject(),
		logProducerAppOnly.K8sObject(),
		logProducerNameOnly.K8sObject(),
		logProducerMixed.K8sObject(),
		&pipeline,
	}
	resources = append(resources, backend.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
	assert.FluentBitLogPipelineHealthy(t, pipelineName)

	assert.DeploymentReady(t, logProducerNone.NamespacedName())
	assert.DeploymentReady(t, logProducerAppOnly.NamespacedName())
	assert.DeploymentReady(t, logProducerNameOnly.NamespacedName())
	assert.DeploymentReady(t, logProducerMixed.NamespacedName())

	assert.FluentBitLogsFromNamespaceDelivered(t, backend, nsNone)
	assert.FluentBitLogsFromNamespaceDelivered(t, backend, nsAppOnly)
	assert.FluentBitLogsFromNamespaceDelivered(t, backend, nsNameOnly)
	assert.FluentBitLogsFromNamespaceDelivered(t, backend, nsMixed)

	// No labels should not have app value
	assert.BackendDataEventuallyMatches(t, backend,
		fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
			fluentbit.HaveNamespace(Equal(nsNone)),
			fluentbit.HaveKubernetesAttributes(Not(HaveKey("app_name"))),
		))),
	)

	// App only label should have app value of app label
	assert.BackendDataEventuallyMatches(t, backend,
		fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
			fluentbit.HaveNamespace(Equal(nsAppOnly)),
			fluentbit.HaveKubernetesAttributes(HaveKeyWithValue("app_name", "app-only")),
		))),
	)

	// Name only label should have app value of name label
	assert.BackendDataEventuallyMatches(t, backend,
		fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
			fluentbit.HaveNamespace(Equal(nsNameOnly)),
			fluentbit.HaveKubernetesAttributes(HaveKeyWithValue("app_name", "name-only")),
		))),
	)

	// Mixed label should have app value of name label
	assert.BackendDataEventuallyMatches(t, backend,
		fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
			fluentbit.HaveNamespace(Equal(nsMixed)),
			fluentbit.HaveKubernetesAttributes(HaveKeyWithValue("app_name", "name-mixed")),
		))),
	)
}
