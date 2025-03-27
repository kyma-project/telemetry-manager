//go:build e2e

package e2e

import (
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/servicenamebundle"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"

	"fmt"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"io"
	"k8s.io/apimachinery/pkg/types"

	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogs, suite.LabelExperimental), Ordered, func() {
	var (
		mockNs           = suite.ID()
		pipelineName     = suite.ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeLogsOtel, backend.WithPersistentHostSecret(suite.IsUpgrade()))

		objs = append(objs, backend.K8sObjects()...)
		objs = append(objs, servicenamebundle.K8sObjects(mockNs, telemetrygen.SignalTypeLogs)...)
		backendExportURL = backend.ExportURL(proxyClient)

		hostSecretRef := backend.HostSecretRefV1Alpha1()
		pipelineBuilder := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithApplicationInput(true).
			WithOTLPOutput(
				testutils.OTLPEndpointFromSecret(
					hostSecretRef.Name,
					hostSecretRef.Namespace,
					hostSecretRef.Key,
				),
			)
		if suite.IsUpgrade() {
			pipelineBuilder.WithLabels(kitk8s.PersistentLabel)
		}
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
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running log gateway deployment", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.LogGatewayName)
		})

		It("Should have a running log agent daemonset", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.LogAgentName)
		})

		It("Should have a log backend running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Should have a running pipeline", func() {
			assert.LogPipelineOtelHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should deliver loggen logs", func() {
			assert.LogsFromNamespaceDelivered(proxyClient, backendExportURL, mockNs)
		})

		verifyServiceNameAttr := func(givenPodPrefix, expectedServiceName string) {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendExportURL)
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
		It("Should set undefined service.name attribute to app.kubernetes.io/name label value", func() {
			verifyServiceNameAttr(servicenamebundle.PodWithBothLabelsName, servicenamebundle.KubeAppLabelValue)
		})

		It("Should set undefined service.name attribute to Job name", func() {
			verifyServiceNameAttr(servicenamebundle.JobName, servicenamebundle.JobName)
		})

		It("Should set undefined service.name attribute to Pod name", func() {
			verifyServiceNameAttr(servicenamebundle.PodWithNoLabelsName, servicenamebundle.PodWithNoLabelsName)
		})
		It("Should have no kyma resource attributes", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendExportURL)
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
