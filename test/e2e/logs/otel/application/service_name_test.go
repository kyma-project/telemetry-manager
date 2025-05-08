//go:build e2e

package otel

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
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogsOtel, suite.LabelExperimental), Ordered, func() {
	var (
		mockNs                = suite.ID()
		pipelineName          = suite.ID()
		backendExportURL      string
		jobName               = "job"
		podWithNoLabelsName   = "pod-with-no-labels"
		kubeAppLabelValue     = "kube-workload"
		appLabelValue         = "workload"
		podWithBothLabelsName = "pod-with-both-app-labels" // #nosec G101 -- This is a false positive

	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeLogsOTel)

		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(suite.ProxyClient)

		hostSecretRef := backend.HostSecretRefV1Alpha1()
		pipelineBuilder := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithOTLPInput(false).
			WithApplicationInput(true).
			WithOTLPOutput(
				testutils.OTLPEndpointFromSecret(
					hostSecretRef.Name,
					hostSecretRef.Namespace,
					hostSecretRef.Key,
				),
			)

		logPipeline := pipelineBuilder.Build()
		objs = append(objs, &logPipeline)

		logs := loggen.New(mockNs)

		objs = append(objs,
			kitk8s.NewPod(podWithBothLabelsName, mockNs).
				WithLabel("app.kubernetes.io/name", kubeAppLabelValue).
				WithLabel("app", appLabelValue).
				WithPodSpec(logs.PodSpec()).
				K8sObject(),
			kitk8s.NewJob(jobName, mockNs).WithPodSpec(logs.PodSpec()).K8sObject(),
			kitk8s.NewPod(podWithNoLabelsName, mockNs).WithPodSpec(logs.PodSpec()).K8sObject(),
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

		It("Should have a running log gateway deployment", func() {
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, kitkyma.LogGatewayName)
		})

		It("Should have a running log agent daemonset", func() {
			assert.DaemonSetReady(suite.Ctx, suite.K8sClient, kitkyma.LogAgentName)
		})

		It("Should have a log backend running", func() {
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Should have a running pipeline", func() {
			assert.OTelLogPipelineHealthy(suite.Ctx, suite.K8sClient, pipelineName)
		})

		It("Should deliver loggen logs", func() {
			assert.OTelLogsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, mockNs)
		})

		verifyServiceNameAttr := func(givenPodPrefix, expectedServiceName string) {
			Eventually(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(bodyContent).To(HaveFlatOTelLogs(
					ContainElement(SatisfyAll(
						HaveResourceAttributes(HaveKeyWithValue("service.name", expectedServiceName)),
						HaveResourceAttributes(HaveKeyWithValue("k8s.pod.name", ContainSubstring(givenPodPrefix))),
					)),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed(), fmt.Sprintf("could not find logs matching service.name: %s, k8s.pod.name: %s.*", expectedServiceName, givenPodPrefix))

		}
		It("Should set undefined service.name attribute to app.kubernetes.io/name label value", func() {
			verifyServiceNameAttr(podWithBothLabelsName, kubeAppLabelValue)
		})

		It("Should set undefined service.name attribute to Job name", func() {
			verifyServiceNameAttr(jobName, jobName)
		})

		It("Should set undefined service.name attribute to Pod name", func() {
			verifyServiceNameAttr(podWithNoLabelsName, podWithNoLabelsName)
		})
		It("Should have no kyma resource attributes", func() {
			Eventually(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(HaveFlatOTelLogs(
					Not(ContainElement(
						HaveResourceAttributes(HaveKey(ContainSubstring("kyma"))),
					)),
				)))
			}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
