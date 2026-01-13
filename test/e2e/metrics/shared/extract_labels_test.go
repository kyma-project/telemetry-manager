package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestExtractLabels(t *testing.T) {
	tests := []struct {
		label            string
		input            telemetryv1beta1.MetricPipelineInput
		generatorBuilder func(ns string, labels map[string]string) []client.Object
	}{
		{
			label: suite.LabelMetricAgentSetA,
			input: testutils.BuildMetricPipelineRuntimeInput(),
			generatorBuilder: func(ns string, labels map[string]string) []client.Object {
				generator := prommetricgen.New(ns)

				return []client.Object{
					generator.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).WithLabels(labels).K8sObject(),
					generator.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
				}
			},
		},
		{
			label: suite.LabelMetricGatewaySetA,
			input: testutils.BuildMetricPipelineOTLPInput(),
			generatorBuilder: func(ns string, labels map[string]string) []client.Object {
				return []client.Object{
					telemetrygen.NewPod(ns, telemetrygen.SignalTypeMetrics).WithLabels(labels).K8sObject(),
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label)

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
				uniquePrefix = unique.Prefix(tc.label)
				pipelineName = uniquePrefix()
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
				telemetry    operatorv1beta1.Telemetry
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)

			pipeline := testutils.NewMetricPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.input).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
				Build()

			genLabels := map[string]string{
				labelKeyExactMatch:     labelValueExactMatch,
				labelKeyPrefixMatch1:   labelValuePrefixMatch1,
				labelKeyPrefixMatch2:   labelValuePrefixMatch2,
				labelKeyShouldNotMatch: labelValueShouldNotMatch,
			}

			kitk8s.PreserveAndScheduleRestoreOfTelemetryResource(t, kitkyma.TelemetryName)

			Eventually(func(g Gomega) {
				g.Expect(suite.K8sClient.Get(t.Context(), kitkyma.TelemetryName, &telemetry)).NotTo(HaveOccurred())
				telemetry.Spec.Enrichments = &operatorv1beta1.EnrichmentSpec{
					ExtractPodLabels: []operatorv1beta1.PodLabel{
						{Key: "metric.test.exact.should.match"},
						{KeyPrefix: "metric.test.prefix"},
					},
				}
				g.Expect(suite.K8sClient.Update(t.Context(), &telemetry)).NotTo(HaveOccurred(), "should update Telemetry resource with enrichment configuration")
			}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				&pipeline,
			}
			resources = append(resources, tc.generatorBuilder(genNs, genLabels)...)
			resources = append(resources, backend.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backend)
			assert.DeploymentReady(t, kitkyma.MetricGatewayName)

			if suite.ExpectAgent(tc.label) {
				assert.DaemonSetReady(t, kitkyma.MetricAgentName)
			}

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
		})
	}
}
