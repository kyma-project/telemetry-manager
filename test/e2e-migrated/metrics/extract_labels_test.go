package metrics

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestExtractLabels(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetricsSetA)

	const (
		k8sLabelKeyPrefix    = "k8s.pod.label"
		metricLabelKeyPrefix = "metric.test.prefix"

		labelKeyExactMatch     = "metric.test.exact.should.match"
		labelKeyPrefixMatch1   = metricLabelKeyPrefix + ".should.match1"
		labelKeyPrefixMatch2   = metricLabelKeyPrefix + ".should.match2"
		labelKeyShouldNotMatch = "metric.test.label.should.not.match"

		labelValueExactMatch     = "exact_match"
		labelValuePrefixMatch1   = "prefix_match1"
		labelValuePrefixMatch2   = "prefix_match2"
		labelValueShouldNotMatch = "should_not_match"
	)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
		telemetry    operatorv1alpha1.Telemetry
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)

	pipeline := testutils.NewMetricPipelineBuilder().
		WithOTLPInput(true).
		WithName(pipelineName).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()

	genLabels := map[string]string{
		labelKeyExactMatch:     labelValueExactMatch,
		labelKeyPrefixMatch1:   labelValuePrefixMatch1,
		labelKeyPrefixMatch2:   labelValuePrefixMatch2,
		labelKeyShouldNotMatch: labelValueShouldNotMatch,
	}

	Expect(suite.K8sClient.Get(t.Context(), kitkyma.TelemetryName, &telemetry)).NotTo(HaveOccurred())
	telemetry.Spec.Enrichments = &operatorv1alpha1.EnrichmentSpec{
		ExtractPodLabels: []operatorv1alpha1.PodLabel{
			{Key: "metric.test.exact.should.match"},
			{KeyPrefix: "metric.test.prefix"},
		},
	}
	Expect(suite.K8sClient.Update(t.Context(), &telemetry)).NotTo(HaveOccurred(), "should update Telemetry resource with enrichment configuration")

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&pipeline,
		telemetrygen.NewPod(genNs, telemetrygen.SignalTypeMetrics).WithLabels(genLabels).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())

		Expect(suite.K8sClient.Get(context.Background(), kitkyma.TelemetryName, &telemetry)).To(Succeed()) //nolint:usetesting // Remove ctx from Get
		telemetry.Spec.Enrichments = &operatorv1alpha1.EnrichmentSpec{}

		Eventually(func(g Gomega) {
			Expect(suite.K8sClient.Update(context.Background(), &telemetry)).To(Succeed()) //nolint:usetesting // Remove ctx from Update
		}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DeploymentReady(t, kitkyma.MetricGatewayName)
	assert.MetricPipelineHealthy(t, pipelineName)

	// Verify that at least one log entry contains the expected labels, rather than requiring all entries to match.
	// This approach accounts for potential delays in the k8sattributes processor syncing with the API server during startup,
	// which can result in some logs not being enriched and causing test flakiness.
	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatMetrics(ContainElement(
			HaveResourceAttributes(SatisfyAll(
				HaveKeyWithValue(k8sLabelKeyPrefix+"."+labelKeyExactMatch, labelValueExactMatch),
				HaveKeyWithValue(k8sLabelKeyPrefix+"."+labelKeyPrefixMatch1, labelValuePrefixMatch1),
				HaveKeyWithValue(k8sLabelKeyPrefix+"."+labelKeyPrefixMatch2, labelValuePrefixMatch2),
				Not(HaveKeyWithValue(k8sLabelKeyPrefix+"."+labelKeyShouldNotMatch, labelValueShouldNotMatch)),
			)),
		)),
	)
}
