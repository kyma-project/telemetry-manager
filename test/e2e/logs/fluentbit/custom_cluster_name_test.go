package fluentbit

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers/log/fluentbit"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestCustomClusterName(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
		telemetry    operatorv1alpha1.Telemetry

		clusterName = "cluster-name"
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit)
	logProducer := stdoutloggen.NewDeployment(genNs)
	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithIncludeContainers(stdoutloggen.DefaultContainerName).
		WithIncludeNamespaces(genNs).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
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

	resources := []client.Object{
		objects.NewNamespace(backendNs).K8sObject(),
		objects.NewNamespace(genNs).K8sObject(),
		&pipeline,
		logProducer.K8sObject(),
		telemetrygen.NewPod(genNs, telemetrygen.SignalTypeMetrics).K8sObject(),
	}
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
	assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
	assert.DeploymentReady(t, logProducer.NamespacedName())
	assert.FluentBitLogPipelineHealthy(t, pipelineName)

	assert.BackendDataEventuallyMatches(t, backend,
		fluentbit.HaveFlatLogs(HaveEach(
			fluentbit.HaveAttributes(HaveKeyWithValue("cluster_identifier", clusterName)),
		)),
		assert.WithOptionalDescription("Should have cluster identifier set by record_modifier filter"),
	)
}
