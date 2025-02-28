//go:build e2e

package logs

import (
	"io"
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
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelLogs, LabelExperimental), Ordered, func() {
	const (
		logLabelExactMatchAttributeKey     = "k8s.pod.label.log.test.exact.should.match"
		logLabelPrefixMatchAttributeKey1   = "k8s.pod.label.log.test.prefix.should.match1"
		logLabelPrefixMatchAttributeKey2   = "k8s.pod.label.log.test.prefix.should.match2"
		logLabelShouldNotMatchAttributeKey = "k8s.pod.label.log.test.label.should.not.match"
	)

	var (
		mockNs           = ID()
		pipelineName     = ID()
		backendExportURL string
	)
	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeLogsOtel, backend.WithPersistentHostSecret(IsUpgrade()))

		logProducer := loggen.New(mockNs)
		objs = append(objs, backend.K8sObjects()...)
		objs = append(objs, logProducer.K8sObject())
		backendExportURL = backend.ExportURL(ProxyClient)

		hostSecretRef := backend.HostSecretRefV1Alpha1()
		pipelineBuilder := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(
				testutils.OTLPEndpointFromSecret(
					hostSecretRef.Name,
					hostSecretRef.Namespace,
					hostSecretRef.Key,
				),
			)
		if IsUpgrade() {
			pipelineBuilder.WithLabels(kitk8s.PersistentLabel)
		}
		logPipeline := pipelineBuilder.Build()
		otlpLogGen := telemetrygen.NewPod(mockNs, telemetrygen.SignalTypeLogs).
			WithLabel("log.test.exact.should.match", "exact_match").
			WithLabel("log.test.prefix.should.match1", "prefix_match1").
			WithLabel("log.test.prefix.should.match2", "prefix_match2").
			WithLabel("log.test.label.should.not.match", "should_not_match").
			K8sObject()
		objs = append(objs, otlpLogGen, &logPipeline)
		return objs
	}

	Context("When a logpipeline with OTLP output exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, k8sObjects...)).Should(Succeed())
		})
		It("Should have global log label config", func() {
			Eventually(func(g Gomega) int {
				var telemetry operatorv1alpha1.Telemetry
				err := K8sClient.Get(Ctx, kitkyma.TelemetryName, &telemetry)
				g.Expect(err).NotTo(HaveOccurred())

				telemetry.Spec.Log = &operatorv1alpha1.LogSpec{
					Enrichments: &operatorv1alpha1.EnrichmentSpec{
						Enabled: true,
						ExtractPodLabels: []operatorv1alpha1.PodLabel{
							{
								Key: "log.test.exact.should.match",
							},
							{
								KeyPrefix: "log.test.prefix",
							},
						},
					},
				}
				err = K8sClient.Update(Ctx, &telemetry)
				g.Expect(err).NotTo(HaveOccurred())
				return len(telemetry.Spec.Log.Enrichments.ExtractPodLabels)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(2))
		})

		It("Should have a running log gateway deployment", func() {
			assert.DeploymentReady(Ctx, K8sClient, kitkyma.LogGatewayName)
		})

		It("Should have a log backend running", func() {
			assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Should have a running pipeline", func() {
			assert.LogPipelineOtelHealthy(Ctx, K8sClient, pipelineName)
		})

		It("Should deliver telemetrygen logs", func() {
			assert.LogsFromNamespaceDelivered(ProxyClient, backendExportURL, mockNs)
		})

		It("Should have a right labels attached to the logs", func() {
			Consistently(func(g Gomega) {
				resp, err := ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(bodyContent).To(HaveFlatOtelLogs(ContainElement(SatisfyAll(
					HaveResourceAttributes(HaveKeyWithValue(logLabelExactMatchAttributeKey, "exact_match")),
					HaveResourceAttributes(HaveKeyWithValue(logLabelPrefixMatchAttributeKey1, "prefix_match1")),
					HaveResourceAttributes(HaveKeyWithValue(logLabelPrefixMatchAttributeKey2, "prefix_match2")),
					Not(HaveResourceAttributes(HaveKeyWithValue(logLabelShouldNotMatchAttributeKey, "should_not_match"))),
				))))
			}, periodic.ConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
