//go:build e2e

package e2e

import (
	"fmt"
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
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), Label(suite.LabelSetA), Ordered, func() {
	var (
		mockNs                                            = suite.ID()
		pipelineName                                      = suite.ID()
		backendExportURL                                  string
		daemonSetName                                     = "daemon-set"
		jobName                                           = "job"
		podWithInvalidStartForUnknownServicePatternName   = "pod-with-invalid-start-for-unknown-service-pattern"
		podWithInvalidEndForUnknownServicePatternName     = "pod-with-invalid-end-for-unknown-service-pattern"
		podWithMissingProcessForUnknownServicePatternName = "pod-with-missing-process-for-unknown-service-pattern"
		attrWithInvalidStartForUnknownServicePattern      = "test_unknown_service"
		attrWithInvalidEndForUnknownServicePattern        = "unknown_service_test"
		attrWithMissingProcessForUnknownServicePattern    = "unknown_service:"
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeMetrics)
		objs = append(objs, backend.K8sObjects()...)

		backendExportURL = backend.ExportURL(proxyClient)

		metricPipeline := testutils.NewMetricPipelineBuilder().
			WithName(pipelineName).
			WithRuntimeInput(true, testutils.IncludeNamespaces(kitkyma.SystemNamespaceName)).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()
		objs = append(objs, &metricPipeline)

		podSpecWithUndefinedService := telemetrygen.PodSpec(signalType,
			telemetrygen.WithServiceName(""))
		podSpecWithInvalidStartForUnknownServicePattern := telemetrygen.PodSpec(signalType,
			telemetrygen.WithServiceName(attrWithInvalidStartForUnknownServicePattern))
		podSpecWithInvalidEndForUnknownServicePattern := telemetrygen.PodSpec(signalType,
			telemetrygen.WithServiceName(attrWithInvalidEndForUnknownServicePattern))
		podSpecWithMissingProcessForUnknownServicePattern := telemetrygen.PodSpec(signalType,
			telemetrygen.WithServiceName(attrWithMissingProcessForUnknownServicePattern))

		objs = append(objs,
			kitk8s.NewDaemonSet(daemonSetName, namespace).WithPodSpec(podSpecWithUndefinedService).K8sObject(),
			kitk8s.NewJob(jobName, namespace).WithPodSpec(podSpecWithUndefinedService).K8sObject(),
			kitk8s.NewPod(podWithInvalidStartForUnknownServicePatternName, namespace).WithPodSpec(podSpecWithInvalidStartForUnknownServicePattern).K8sObject(),
			kitk8s.NewPod(podWithInvalidEndForUnknownServicePatternName, namespace).WithPodSpec(podSpecWithInvalidEndForUnknownServicePattern).K8sObject(),
			kitk8s.NewPod(podWithMissingProcessForUnknownServicePatternName, namespace).WithPodSpec(podSpecWithMissingProcessForUnknownServicePattern).K8sObject(),
		)

		return objs
	}

	Context("When a MetricPipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running metric gateway deployment", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.MetricGatewayName)

		})

		It("Ensures the metric agent daemonset is ready", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		It("Should have a metrics backend running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
			assert.ServiceReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Should have a running pipeline", func() {
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineName)
		})

		verifyServiceNameAttr := func(givenPodPrefix, expectedServiceName string) {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(bodyContent).To(HaveFlatMetrics(
					ContainElement(SatisfyAll(
						HaveResourceAttributes(HaveKeyWithValue("service.name", expectedServiceName)),
						HaveResourceAttributes(HaveKeyWithValue("k8s.pod.name", ContainSubstring(givenPodPrefix))),
					)),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed(), fmt.Sprintf("could not find metrics matching service.name: %s, k8s.pod.name: %s.*", expectedServiceName, givenPodPrefix))
		}

		It("Should set undefined service.name attribute to DaemonSet name", func() {
			verifyServiceNameAttr(daemonSetName, daemonSetName)
		})

		It("Should set undefined service.name attribute to Job name", func() {
			verifyServiceNameAttr(jobName, jobName)
		})

		It("Should NOT enrich service.name attribute when its value is not following the unknown_service:<process.executable.name> pattern", func() {
			verifyServiceNameAttr(podWithInvalidStartForUnknownServicePatternName, attrWithInvalidStartForUnknownServicePattern)
			verifyServiceNameAttr(podWithInvalidEndForUnknownServicePatternName, attrWithInvalidEndForUnknownServicePattern)
			verifyServiceNameAttr(podWithMissingProcessForUnknownServicePatternName, attrWithMissingProcessForUnknownServicePattern)
		})

		It("Should have metrics with service.name set to telemetry-metric-gateway", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(bodyContent).To(HaveFlatMetrics(
					ContainElement(HaveResourceAttributes(HaveKeyWithValue("service.name", kitkyma.MetricGatewayBaseName))),
				))
			}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should have metrics with service.name set to telemetry-metric-agent", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(bodyContent).To(HaveFlatMetrics(
					ContainElement(HaveResourceAttributes(HaveKeyWithValue("service.name", kitkyma.MetricAgentBaseName))),
				))
			}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
