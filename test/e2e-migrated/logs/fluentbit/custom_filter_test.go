package fluentbit

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestCustomFilterDenied(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix("denied")
	)

	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithCustomFilter("Random custom filter").
		WithCustomOutput("Random custom output").
		Build()

	Consistently(func(g Gomega) {
		g.Expect(kitk8s.CreateObjects(t.Context(), &pipeline)).ShouldNot(Succeed())
	}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func TestCustomFilterAllowed(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix("allowed")
		backendNs    = uniquePrefix("backend")
		includeNs    = uniquePrefix("include")
		excludeNs    = uniquePrefix("exclude")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit)
	logProducerInclude := stdloggen.NewDeployment(includeNs)
	logProducerExclude := stdloggen.NewDeployment(excludeNs)
	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithCustomFilter(fmt.Sprintf(`
	    Name    grep
	    Regex   $kubernetes['namespace_name'] %s`, includeNs)).
		WithCustomFilter(fmt.Sprintf(`
	    Name    grep
	    Exclude $kubernetes['namespace_name'] %s`, excludeNs)).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port()), testutils.HTTPDedot(true)).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(includeNs).K8sObject(),
		kitk8s.NewNamespace(excludeNs).K8sObject(),
		logProducerInclude.K8sObject(),
		logProducerExclude.K8sObject(),
		&pipeline,
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

	assert.FluentBitLogPipelineHealthy(t, pipelineName)
	assert.LogPipelineUnsupportedMode(t, pipelineName, true)
	assert.DaemonSetReady(t.Context(), kitkyma.FluentBitDaemonSetName)
	assert.DeploymentReady(t.Context(), backend.NamespacedName())
	assert.FluentBitLogsFromNamespaceDelivered(t, backend, includeNs)
	assert.FluentBitLogsFromNamespaceNotDelivered(t, backend, excludeNs)
}
