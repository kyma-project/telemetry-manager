//go:build e2e

package otlp

import (
	"fmt"
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogsOtel, suite.LabelExperimental), Ordered, func() {
	var (
		mockNs              = suite.ID()
		pipelineName        = suite.ID()
		backendExportURL    string
		podWithAppLabelName = "pod-with-app-label"
		statefulSetName     = "stateful-set"
		appLabelValue       = "workload"
		deploymentName      = "deployment"
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())
		backend := backend.New(mockNs, backend.SignalTypeLogsOtel)
		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(suite.ProxyClient)

		hostSecretRef := backend.HostSecretRefV1Alpha1()
		pipelineBuilder := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithApplicationInput(false).
			WithKeepOriginalBody(false).
			WithOTLPOutput(
				testutils.OTLPEndpointFromSecret(
					hostSecretRef.Name,
					hostSecretRef.Namespace,
					hostSecretRef.Key,
				),
			)

		logPipeline := pipelineBuilder.Build()

		objs = append(objs, &logPipeline)

		podSpecWithUndefinedService := telemetrygen.PodSpec(telemetrygen.SignalTypeLogs,
			telemetrygen.WithServiceName(""))

		objs = append(objs,
			kitk8s.NewPod(podWithAppLabelName, mockNs).
				WithLabel("app", appLabelValue).
				WithPodSpec(podSpecWithUndefinedService).
				K8sObject(),
			kitk8s.NewDeployment(deploymentName, mockNs).WithPodSpec(podSpecWithUndefinedService).K8sObject(),
			kitk8s.NewStatefulSet(statefulSetName, mockNs).WithPodSpec(podSpecWithUndefinedService).K8sObject(),
		)

		return objs

	}

	Context("When a Log Pipeline with otlp output exists", Ordered, func() {

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

		It("Should have 2 log gateway replicas", func() {
			Eventually(func(g Gomega) int32 {
				var deployment appsv1.Deployment
				err := suite.K8sClient.Get(suite.Ctx, kitkyma.LogGatewayName, &deployment)
				g.Expect(err).NotTo(HaveOccurred())
				return *deployment.Spec.Replicas
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(int32(2)))
		})

		It("Should have a log backend running", func() {
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Should have a running pipeline", func() {
			assert.LogPipelineOtelHealthy(suite.Ctx, suite.K8sClient, pipelineName)
		})

		It("Should deliver telemetrygen logs", func() {
			assert.OtelLogsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, mockNs)
		})

		verifyServiceNameAttr := func(givenPodPrefix, expectedServiceName string) {
			Eventually(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(bodyContent).To(HaveFlatOtelLogs(
					ContainElement(SatisfyAll(
						HaveResourceAttributes(HaveKeyWithValue("service.name", expectedServiceName)),
						HaveResourceAttributes(HaveKeyWithValue("k8s.pod.name", ContainSubstring(givenPodPrefix))),
					)),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed(), fmt.Sprintf("could not find logs matching service.name: %s, k8s.pod.name: %s.*", expectedServiceName, givenPodPrefix))

		}

		It("Should set undefined service.name attribute to app label value", func() {
			verifyServiceNameAttr(podWithAppLabelName, appLabelValue)
		})

		It("Should set undefined service.name attribute to Deployment name", func() {
			verifyServiceNameAttr(deploymentName, deploymentName)
		})

		It("Should set undefined service.name attribute to StatefulSet name", func() {
			verifyServiceNameAttr(statefulSetName, statefulSetName)
		})

		It("Should have no kyma resource attributes", func() {
			Eventually(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(HaveFlatOtelLogs(
					Not(ContainElement(
						HaveResourceAttributes(HaveKey(ContainSubstring("kyma"))),
					)),
				)))
			}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

	})
})
