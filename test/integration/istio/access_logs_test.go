package istio

import (
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	"github.com/kyma-project/telemetry-manager/test/testkit/istio"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log/fluentbit"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestAccessLogsFluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelGardener, suite.LabelIstio, suite.LabelFluentBit)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit)

	logPipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithIncludeContainers("istio-proxy").
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
		Build()

	sampleApp := prommetricgen.New(permissiveNs, prommetricgen.WithName("access-log-emitter"))
	metricPodURL := suite.ProxyClient.ProxyURLForPod(permissiveNs, sampleApp.Name(), sampleApp.MetricsEndpoint(), sampleApp.MetricsPort())

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		&logPipeline,
		sampleApp.Pod().K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	require.NoError(t, kitk8s.CreateObjects(t, resources...))

	assert.BackendReachable(t, backend)

	listOptions := client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{"app.kubernetes.io/name": "metric-producer"}),
		Namespace:     permissiveNs,
	}
	assert.PodsReady(t, listOptions)
	assert.FluentBitLogPipelineHealthy(t, pipelineName)
	assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)

	Eventually(func(g Gomega) {
		resp, err := suite.ProxyClient.Get(metricPodURL)
		g.Expect(err).NotTo(HaveOccurred())
		resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed(),
		"Should invoke the metrics endpoint to generate access logs",
	)

	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatLogs(ContainElement(
			HaveAttributes(HaveKey(BeElementOf(istio.AccessLogAttributeKeys))),
		)),
		assert.WithOptionalDescription("Istio access logs should be present"),
	)
}
