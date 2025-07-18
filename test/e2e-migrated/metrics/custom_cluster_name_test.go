package metrics

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
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

func TestCustomClusterName(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetricsSetA)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
		telemetry    operatorv1alpha1.Telemetry
		kubeSystemNs corev1.Namespace

		clusterName = "cluster-name"
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)

	pipeline := testutils.NewMetricPipelineBuilder().
		WithOTLPInput(true).
		WithName(pipelineName).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()

	Expect(suite.K8sClient.Get(t.Context(), kitkyma.TelemetryName, &telemetry)).NotTo(HaveOccurred())
	telemetry.Spec.Enrichments = &operatorv1alpha1.EnrichmentSpec{
		Cluster: &operatorv1alpha1.Cluster{
			Name: clusterName,
		},
	}
	Expect(suite.K8sClient.Update(t.Context(), &telemetry)).NotTo(HaveOccurred(), "should update Telemetry resource with cluster name")

	Expect(suite.K8sClient.Get(t.Context(), types.NamespacedName{Name: "kube-system"}, &kubeSystemNs)).NotTo(HaveOccurred(), "should get the kube-system namespace")
	clusterUID := string(kubeSystemNs.UID)

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&pipeline,
		telemetrygen.NewPod(genNs, telemetrygen.SignalTypeMetrics).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(resources...))

		Expect(suite.K8sClient.Get(context.Background(), kitkyma.TelemetryName, &telemetry)).Should(Succeed()) //nolint:usetesting // Remove ctx from Get
		telemetry.Spec.Enrichments.Cluster = &operatorv1alpha1.Cluster{}
		Eventually(func(g Gomega) {
			Expect(suite.K8sClient.Update(context.Background(), &telemetry)).To(Succeed()) //nolint:usetesting // Remove ctx from Update
		}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())

	})
	Expect(kitk8s.CreateObjects(t, resources...)).Should(Succeed())

	assert.BackendReachable(t, backend)
	assert.DeploymentReady(t, kitkyma.MetricGatewayName)
	assert.MetricPipelineHealthy(t, pipelineName)

	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatMetrics(ContainElement(
			HaveResourceAttributes(SatisfyAll(
				HaveKeyWithValue("k8s.cluster.name", clusterName),
				HaveKeyWithValue("k8s.cluster.uid", clusterUID),
			)),
		)),
	)
}
