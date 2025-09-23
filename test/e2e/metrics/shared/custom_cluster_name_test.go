package shared

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
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

func TestCustomClusterName(t *testing.T) {
	tests := []struct {
		label            string
		input            telemetryv1alpha1.MetricPipelineInput
		generatorBuilder func(ns string) []client.Object
	}{
		{
			label: suite.LabelMetricAgent,
			input: testutils.BuildMetricPipelineRuntimeInput(),
			generatorBuilder: func(ns string) []client.Object {
				generator := prommetricgen.New(ns)

				return []client.Object{
					generator.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
					generator.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
				}
			},
		},
		{
			label: suite.LabelMetricGateway,
			input: testutils.BuildMetricPipelineOTLPInput(),
			generatorBuilder: func(ns string) []client.Object {
				return []client.Object{
					telemetrygen.NewPod(ns, telemetrygen.SignalTypeMetrics).K8sObject(),
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label)

			var (
				uniquePrefix = unique.Prefix(tc.label)
				pipelineName = uniquePrefix()
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
				telemetry    operatorv1alpha1.Telemetry
				kubeSystemNs corev1.Namespace

				clusterName = "cluster-name"
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)

			pipeline := testutils.NewMetricPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.input).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
				Build()

			Eventually(func(g Gomega) {
				g.Expect(suite.K8sClient.Get(t.Context(), kitkyma.TelemetryName, &telemetry)).NotTo(HaveOccurred())
				telemetry.Spec.Enrichments = &operatorv1alpha1.EnrichmentSpec{
					Cluster: &operatorv1alpha1.Cluster{
						Name: clusterName,
					},
				}
				g.Expect(suite.K8sClient.Update(t.Context(), &telemetry)).NotTo(HaveOccurred(), "should update Telemetry resource with cluster name")
			}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())

			Expect(suite.K8sClient.Get(t.Context(), types.NamespacedName{Name: "kube-system"}, &kubeSystemNs)).NotTo(HaveOccurred(), "should get the kube-system namespace")
			clusterUID := string(kubeSystemNs.UID)

			resources := []client.Object{
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(genNs).K8sObject(),
				&pipeline,
			}
			resources = append(resources, tc.generatorBuilder(genNs)...)
			resources = append(resources, backend.K8sObjects()...)

			t.Cleanup(func() {
				Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(suite.K8sClient.Get(context.Background(), kitkyma.TelemetryName, &telemetry)).Should(Succeed()) //nolint:usetesting // Remove ctx from Get
					telemetry.Spec.Enrichments.Cluster = &operatorv1alpha1.Cluster{}
					g.Expect(suite.K8sClient.Update(context.Background(), &telemetry)).To(Succeed()) //nolint:usetesting // Remove ctx from Update
				}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(t, resources...)).Should(Succeed())

			assert.BackendReachable(t, backend)
			assert.DeploymentReady(t, kitkyma.MetricGatewayName)

			if tc.label == suite.LabelMetricAgent {
				assert.DaemonSetReady(t, kitkyma.MetricAgentName)
			}

			assert.MetricPipelineHealthy(t, pipelineName)

			assert.BackendDataEventuallyMatches(t, backend,
				HaveFlatMetrics(ContainElement(
					HaveResourceAttributes(SatisfyAll(
						HaveKeyWithValue("k8s.cluster.name", clusterName),
						HaveKeyWithValue("k8s.cluster.uid", clusterUID),
					)),
				)),
			)
		})
	}
}
