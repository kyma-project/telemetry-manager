//go:build e2e

package otel

import (
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogsOtel, suite.LabelSignalPull, suite.LabelExperimental), Ordered, func() {
	var (
		mockNs           = suite.ID()
		backendNs        = suite.IDWithSuffix("backend")
		pipelineName     = suite.ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())
		objs = append(objs, kitk8s.NewNamespace(backendNs).K8sObject())

		backend := backend.New(backendNs, backend.SignalTypeLogsOtel)
		logProducer := loggen.New(mockNs).WithUseJSON()
		objs = append(objs, backend.K8sObjects()...)
		objs = append(objs, logProducer.K8sObject())
		backendExportURL = backend.ExportURL(suite.ProxyClient)

		hostSecretRef := backend.HostSecretRefV1Alpha1()
		pipelineBuilder := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithIncludeNamespaces(mockNs).
			WithApplicationInput(true).
			WithOTLPOutput(
				testutils.OTLPEndpointFromSecret(
					hostSecretRef.Name,
					hostSecretRef.Namespace,
					hostSecretRef.Key,
				),
			)
		logPipeline := pipelineBuilder.Build()

		objs = append(objs,
			&logPipeline,
		)
		return objs
	}

	Context("When a log pipeline with runtime input exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running log agent daemonset", func() {
			assert.DaemonSetReady(suite.Ctx, suite.K8sClient, kitkyma.LogAgentName)
		})

		It("Should have a log backend running", func() {
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: backendNs})
		})

		It("Should have a running pipeline", func() {
			assert.LogPipelineOtelHealthy(suite.Ctx, suite.K8sClient, pipelineName)
		})

		It("Should deliver loggen logs", func() {
			assert.OtelLogsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, mockNs)
		})

		It("Should have trace and span ids in logs", func() {
			Eventually(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(bodyContent).To(HaveFlatOtelLogs(ContainElement(SatisfyAll(
					HaveOtelTimestamp(Not(BeEmpty())),
					HaveObservedTimestamp(Not(BeEmpty())),
					HaveTraceId(Equal("255c2212dd02c02ac59a923ff07aec74")),
					HaveSpanId(Equal("c5c735f175ad06a6")),
					HaveTraceFlags(Equal("01")),
					HaveAttributes(Not(HaveKey("trace_id"))),
					HaveAttributes(Not(HaveKey("span_id"))),
					HaveAttributes(Not(HaveKey("trace_flags"))),
					HaveAttributes(Not(HaveKey("traceparent"))),
				))))
			}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should parse traceparent", func() {
			Eventually(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(bodyContent).To(HaveFlatOtelLogs(ContainElement(SatisfyAll(
					HaveOtelTimestamp(Not(BeEmpty())),
					HaveObservedTimestamp(Not(BeEmpty())),
					HaveTraceId(Equal("80e1afed08e019fc1110464cfa66635c")),
					HaveSpanId(Equal("7a085853722dc6d2")),
					HaveTraceFlags(Equal("01")),
					HaveAttributes(Not(HaveKey("trace_id"))),
					HaveAttributes(Not(HaveKey("span_id"))),
					HaveAttributes(Not(HaveKey("trace_flags"))),
					HaveAttributes(Not(HaveKey("traceparent"))),
				))))
			}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should have span_id log attribute but no trace data, not parsable", func() {
			Consistently(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(bodyContent).To(HaveFlatOtelLogs(ContainElement(SatisfyAll(
					HaveOtelTimestamp(Not(BeEmpty())),
					HaveObservedTimestamp(Not(BeEmpty())),
					HaveTraceId(BeEmpty()),
					HaveSpanId(BeEmpty()),
					HaveAttributes(HaveKey("span_id")),
				))))
			}, periodic.ConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should have severityText and severityNumber in logs", func() {
			Eventually(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(bodyContent).To(HaveFlatOtelLogs(ContainElement(SatisfyAll(
					HaveTraceId(Equal("80e1afed08e019fc1110464cfa66635c")),
					HaveSeverityNumber(Equal(6)),
					HaveSeverityText(Equal("WARN")),
					aveAttributes(Not(HaveKey("log.level"))),
				))))

				g.Expect(bodyContent).To(HaveFlatOtelLogs(ContainElement(SatisfyAll(
					HaveTraceId(Equal("255c2212dd02c02ac59a923ff07aec74")),
					HaveSeverityNumber(Equal(8)),
					HaveSeverityText(Equal("INFO")),
					HaveAttributes(Not(HaveKey("level"))),
				))))
			}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
