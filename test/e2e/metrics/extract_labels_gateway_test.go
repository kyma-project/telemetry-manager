//go:build e2e

package metrics

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics, suite.LabelExperimental), Ordered, func() {
	const (
		k8sLabelKeyPrefix = "k8s.pod.label"
		logLabelKeyPrefix = "metric.test.prefix"

		labelKeyExactMatch     = "metric.test.exact.should.match"
		labelKeyPrefixMatch1   = logLabelKeyPrefix + ".should.match1"
		labelKeyPrefixMatch2   = logLabelKeyPrefix + ".should.match2"
		labelKeyShouldNotMatch = "metric.test.label.should.not.match"

		labelValueExactMatch     = "exact_match"
		labelValuePrefixMatch1   = "prefix_match1"
		labelValuePrefixMatch2   = "prefix_match2"
		labelValueShouldNotMatch = "should_not_match"
	)

	var (
		mockNs           = suite.ID()
		pipelineName     = suite.ID()
		backend          *kitbackend.Backend
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend = kitbackend.New(mockNs, kitbackend.SignalTypeMetrics)
		backendExportURL = backend.ExportURL(suite.ProxyClient)
		objs = append(objs, backend.K8sObjects()...)

		metricPipeline := testutils.NewMetricPipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).Build()

		genLabels := map[string]string{
			labelKeyExactMatch:     labelValueExactMatch,
			labelKeyPrefixMatch1:   labelValuePrefixMatch1,
			labelKeyPrefixMatch2:   labelValuePrefixMatch2,
			labelKeyShouldNotMatch: labelValueShouldNotMatch,
		}

		objs = append(objs, telemetrygen.NewPod(mockNs, telemetrygen.SignalTypeMetrics).WithLabels(genLabels).K8sObject(),
			&metricPipeline,
		)

		return objs
	}

	Context("When a metricpipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(suite.Ctx, k8sObjects...)).Should(Succeed())
				resetTelemetryToDefault()

			})

			Expect(kitk8s.CreateObjects(suite.Ctx, k8sObjects...)).Should(Succeed())

		})

		It("Should have a running metric gateway deployment", func() {
			assert.DeploymentReady(suite.Ctx, kitkyma.MetricGatewayName)
		})

		It("Should configure label enrichments", func() {
			var telemetry operatorv1alpha1.Telemetry
			err := suite.K8sClient.Get(suite.Ctx, kitkyma.TelemetryName, &telemetry)
			Expect(err).NotTo(HaveOccurred())

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

			err = suite.K8sClient.Update(suite.Ctx, &telemetry)
			Expect(err).To(Not(HaveOccurred()))
		})

		It("Should have a metrics backend running", func() {
			assert.DeploymentReady(suite.Ctx, types.NamespacedName{Name: kitbackend.DefaultName, Namespace: mockNs})
		})

		It("Should have a running pipeline", func() {
			assert.MetricPipelineHealthy(suite.Ctx, pipelineName)
		})

		It("Should deliver telemetrygen metrics", func() {
			assert.MetricsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, mockNs, telemetrygen.MetricNames)
		})

		It("Should verify label enrichments for metrics", func() {
			Eventually(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(HaveFlatMetrics(ContainElement(
					HaveResourceAttributes(SatisfyAll(
						HaveKeyWithValue(k8sLabelKeyPrefix+"."+labelKeyExactMatch, labelValueExactMatch),
						HaveKeyWithValue(k8sLabelKeyPrefix+"."+labelKeyPrefixMatch1, labelValuePrefixMatch1),
						HaveKeyWithValue(k8sLabelKeyPrefix+"."+labelKeyPrefixMatch2, labelValuePrefixMatch2),
						Not(HaveKeyWithValue(k8sLabelKeyPrefix+"."+labelKeyShouldNotMatch, labelValueShouldNotMatch)),
					)),
				))))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

	})
})
