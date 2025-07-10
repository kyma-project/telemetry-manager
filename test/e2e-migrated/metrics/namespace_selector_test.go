package metrics

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
	"github.com/kyma-project/telemetry-manager/test/testkit/metrics/runtime"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestNamespaceSelector(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetricsSetB)

	var (
		uniquePrefix            = unique.Prefix()
		app1Ns                  = uniquePrefix("app1")
		app2Ns                  = uniquePrefix("app2")
		backendNs               = uniquePrefix("backend")
		backend1Name            = uniquePrefix("backend1")
		backend2Name            = uniquePrefix("backend2")
		pipelineNameIncludeApp1 = uniquePrefix("include-app1")
		pipelineNameExcludeApp1 = uniquePrefix("exclude-app1")
	)

	backend1 := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName(backend1Name))
	backend2 := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName(backend2Name))

	pipelineIncludeApp1Ns := testutils.NewMetricPipelineBuilder().
		WithName(pipelineNameIncludeApp1).
		WithPrometheusInput(true, testutils.IncludeNamespaces(app1Ns)).
		WithRuntimeInput(true, testutils.IncludeNamespaces(app1Ns)).
		WithOTLPInput(true, testutils.IncludeNamespaces(app1Ns)).
		WithOTLPOutput(testutils.OTLPEndpoint(backend1.Endpoint())).
		Build()

	pipelineExcludeApp1Ns := testutils.NewMetricPipelineBuilder().
		WithName(pipelineNameExcludeApp1).
		WithPrometheusInput(true, testutils.ExcludeNamespaces(app1Ns)).
		WithRuntimeInput(true, testutils.ExcludeNamespaces(app1Ns)).
		WithOTLPInput(true, testutils.ExcludeNamespaces(app1Ns)).
		WithOTLPOutput(testutils.OTLPEndpoint(backend2.Endpoint())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(app1Ns).K8sObject(),
		kitk8s.NewNamespace(app2Ns).K8sObject(),
		kitk8s.NewNamespace(backendNs).K8sObject(),
		&pipelineIncludeApp1Ns,
		&pipelineExcludeApp1Ns,
		telemetrygen.NewPod(app1Ns, telemetrygen.SignalTypeMetrics).K8sObject(),
		telemetrygen.NewPod(app2Ns, telemetrygen.SignalTypeMetrics).K8sObject(),
		telemetrygen.NewPod(kitkyma.SystemNamespaceName, telemetrygen.SignalTypeMetrics).K8sObject(),
		prommetricgen.New(app1Ns).Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		prommetricgen.New(app2Ns).Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		prommetricgen.New(kitkyma.SystemNamespaceName).Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
	}
	resources = append(resources, backend1.K8sObjects()...)
	resources = append(resources, backend2.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

	assert.DeploymentReady(t.Context(), kitkyma.MetricGatewayName)
	assert.DaemonSetReady(t.Context(), kitkyma.MetricAgentName)
	assert.DeploymentReady(t.Context(), backend1.NamespacedName())
	assert.DeploymentReady(t.Context(), backend2.NamespacedName())

	assert.MetricsFromNamespaceDelivered(t, backend1, app1Ns, runtime.DefaultMetricsNames)
	assert.MetricsFromNamespaceDelivered(t, backend1, app1Ns, prommetricgen.CustomMetricNames())
	assert.MetricsFromNamespaceDelivered(t, backend1, app1Ns, telemetrygen.MetricNames)
	assert.MetricsFromNamespaceNotDelivered(t, backend1, app2Ns)
	assert.MetricsFromNamespaceDelivered(t, backend2, app2Ns, runtime.DefaultMetricsNames)
	assert.MetricsFromNamespaceDelivered(t, backend2, app2Ns, prommetricgen.CustomMetricNames())
	assert.MetricsFromNamespaceDelivered(t, backend2, app2Ns, telemetrygen.MetricNames)
	assert.MetricsFromNamespaceNotDelivered(t, backend2, app1Ns)
}
