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
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

var diagnosticMetrics = []any{
	"up",
	"scrape_duration_seconds",
	"scrape_samples_scraped",
	"scrape_samples_post_metric_relabeling",
	"scrape_series_added",
}

func TestPrometheusInputDiagnosticMetric(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetrics)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)
	metricProducer := prommetricgen.New(genNs)

	pipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		WithPrometheusInput(true).
		WithPrometheusInputDiagnosticMetrics(true).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&pipeline,
		metricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		metricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

	assert.DeploymentReady(t.Context(), kitkyma.MetricGatewayName)
	assert.DeploymentReady(t.Context(), backend.NamespacedName())
	assert.DaemonSetReady(t.Context(), kitkyma.MetricAgentName)
	assert.MetricPipelineHealthy(t.Context(), pipelineName)
	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatMetrics(HaveUniqueNames(
			ContainElements(diagnosticMetrics...))))
}
