package shared

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestExtractLabels_OTel(t *testing.T) {
	tests := []struct {
		label                  string
		metricGeneratorBuilder func(ns string, labels map[string]string) client.Object
		expectAgent            bool
	}{
		{
			label: suite.LabelMetricAgent,
			metricGeneratorBuilder: func(ns string, labels map[string]string) client.Object {
				return prommetricgen.New(ns).Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).
					WithLabels(labels).
					K8sObject()
			},
			expectAgent: true,
		},
		{
			label: suite.LabelMetricGateway,

			metricGeneratorBuilder: func(ns string, labels map[string]string) client.Object {
				return telemetrygen.NewPod(ns, telemetrygen.SignalTypeMetrics).WithLabels(labels).K8sObject()
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
				backendNs    = uniquePrefix("backend")

				genNs        = uniquePrefix("gen")
				pipelineName = uniquePrefix()
				telemetry    operatorv1alpha1.Telemetry
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)

			pipelineBuilder := testutils.NewMetricPipelineBuilder().
				WithName(pipelineName).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint()))

			if tc.expectAgent {
				pipelineBuilder.WithPrometheusInput(true, testutils.IncludeNamespaces(genNs))
			}

			pipeline := pipelineBuilder.Build()

			genLabels := map[string]string{
				labelKeyExactMatch:     labelValueExactMatch,
				labelKeyPrefixMatch1:   labelValuePrefixMatch1,
				labelKeyPrefixMatch2:   labelValuePrefixMatch2,
				labelKeyShouldNotMatch: labelValueShouldNotMatch,
			}

			Eventually(func(g Gomega) int {
				err := suite.K8sClient.Get(t.Context(), kitkyma.TelemetryName, &telemetry)
				g.Expect(err).NotTo(HaveOccurred())

				telemetry.Spec.Enrichments = &operatorv1alpha1.EnrichmentSpec{
					ExtractPodLabels: []operatorv1alpha1.PodLabel{
						{
							Key: "metric.test.exact.should.match",
						},
						{
							KeyPrefix: "metric.test.prefix",
						},
					},
				}
				err = suite.K8sClient.Update(t.Context(), &telemetry)
				g.Expect(err).NotTo(HaveOccurred())
				return len(telemetry.Spec.Enrichments.ExtractPodLabels)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(2))

			resources := []client.Object{
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(genNs).K8sObject(),
				&pipeline,
				tc.metricGeneratorBuilder(genNs, genLabels),
			}
			resources = append(resources, backend.K8sObjects()...)

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects

				Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.TelemetryName, &telemetry)).Should(Succeed())
				telemetry.Spec.Enrichments = &operatorv1alpha1.EnrichmentSpec{}
				require.NoError(t, suite.K8sClient.Update(context.Background(), &telemetry)) //nolint:usetesting // Remove ctx from Update
			})
			Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

			if tc.expectAgent {
				assert.DaemonSetReady(t.Context(), kitkyma.MetricAgentName)
			}

			assert.DeploymentReady(t.Context(), kitkyma.MetricGatewayName)
			assert.DeploymentReady(t.Context(), backend.NamespacedName())
			assert.MetricPipelineHealthy(t.Context(), pipelineName)

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
