//go:build e2e

package traces

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
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/trace"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelTraces), Ordered, func() {
	const (
		k8sLabelKeyPrefix = "k8s.pod.label"
		logLabelKeyPrefix = "trace.test.prefix"

		labelKeyExactMatch     = "trace.test.exact.should.match"
		labelKeyPrefixMatch1   = logLabelKeyPrefix + ".should.match1"
		labelKeyPrefixMatch2   = logLabelKeyPrefix + ".should.match2"
		labelKeyShouldNotMatch = "trace.test.label.should.not.match"

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

		backend = kitbackend.New(mockNs, kitbackend.SignalTypeTraces)
		backendExportURL = backend.ExportURL(suite.ProxyClient)
		objs = append(objs, backend.K8sObjects()...)

		tracePipeline := testutils.NewTracePipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).Build()

		genLabels := map[string]string{
			labelKeyExactMatch:     labelValueExactMatch,
			labelKeyPrefixMatch1:   labelValuePrefixMatch1,
			labelKeyPrefixMatch2:   labelValuePrefixMatch2,
			labelKeyShouldNotMatch: labelValueShouldNotMatch,
		}

		objs = append(objs, telemetrygen.NewPod(mockNs, telemetrygen.SignalTypeTraces).WithLabels(genLabels).K8sObject(),
			&tracePipeline,
		)

		return objs
	}

	Context("When a tracepipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(GinkgoT(), k8sObjects...)).Should(Succeed())
				resetTelemetryToDefault()

			})

			Expect(kitk8s.CreateObjects(GinkgoT(), k8sObjects...)).Should(Succeed())

		})

		It("Should have a running trace gateway deployment", func() {
			assert.DeploymentReady(GinkgoT(), kitkyma.TraceGatewayName)
		})

		It("Should configure label enrichments", func() {
			var telemetry operatorv1alpha1.Telemetry
			err := suite.K8sClient.Get(suite.Ctx, kitkyma.TelemetryName, &telemetry)
			Expect(err).NotTo(HaveOccurred())

			telemetry.Spec.Enrichments = &operatorv1alpha1.EnrichmentSpec{
				ExtractPodLabels: []operatorv1alpha1.PodLabel{
					{
						Key: "trace.test.exact.should.match",
					},
					{
						KeyPrefix: "trace.test.prefix",
					},
				},
			}

			err = suite.K8sClient.Update(suite.Ctx, &telemetry)
			Expect(err).To(Not(HaveOccurred()))
		})

		It("Should have a trace backend running", func() {
			assert.DeploymentReady(GinkgoT(), types.NamespacedName{Name: kitbackend.DefaultName, Namespace: mockNs})
		})

		It("Should have a running trace pipeline", func() {
			assert.TracePipelineHealthy(GinkgoT(), pipelineName)
		})

		It("Should deliver telemetrygen metrics", func() {
			assert.TracesFromNamespaceDelivered(suite.ProxyClient, backendExportURL, mockNs)
		})

		It("Should verify label enrichments for traces", func() {
			Eventually(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(HaveFlatTraces(ContainElement(
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

func resetTelemetryToDefault() {
	var telemetry operatorv1alpha1.Telemetry

	Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.TelemetryName, &telemetry)).Should(Succeed())
	telemetry.Spec.Enrichments = &operatorv1alpha1.EnrichmentSpec{}
	Expect(suite.K8sClient.Update(suite.Ctx, &telemetry)).Should(Succeed())
}
