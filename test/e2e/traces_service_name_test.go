//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/servicenamebundle"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelTraces), func() {
	var (
		mockNs           = suite.ID()
		pipelineName     = suite.ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeTraces)
		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(proxyClient)

		tracePipeline := testutils.NewTracePipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()
		objs = append(objs, &tracePipeline)

		objs = append(objs, servicenamebundle.K8sObjects(mockNs, telemetrygen.SignalTypeTraces)...)
		return objs
	}

	Context("When a TracePipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running trace gateway deployment", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.TraceGatewayName)

		})

		It("Should have a trace backend running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Should have a running pipeline", func() {
			assert.TracePipelineHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should set undefined service.name attribute to app.kubernetes.io/name label value", func() {
			assert.VerifyServiceNameAttr(proxyClient, backendExportURL, servicenamebundle.PodWithBothLabelsName, servicenamebundle.KubeAppLabelValue)
		})

		It("Should set undefined service.name attribute to app label value", func() {
			assert.VerifyServiceNameAttr(proxyClient, backendExportURL, servicenamebundle.PodWithAppLabelName, servicenamebundle.AppLabelValue)
		})

		It("Should set undefined service.name attribute to Deployment name", func() {
			assert.VerifyServiceNameAttr(proxyClient, backendExportURL, servicenamebundle.DeploymentName, servicenamebundle.DeploymentName)
		})

		It("Should set undefined service.name attribute to StatefulSet name", func() {
			assert.VerifyServiceNameAttr(proxyClient, backendExportURL, servicenamebundle.StatefulSetName, servicenamebundle.StatefulSetName)
		})

		It("Should set undefined service.name attribute to DaemonSet name", func() {
			assert.VerifyServiceNameAttr(proxyClient, backendExportURL, servicenamebundle.DaemonSetName, servicenamebundle.DaemonSetName)
		})

		It("Should set undefined service.name attribute to Job name", func() {
			assert.VerifyServiceNameAttr(proxyClient, backendExportURL, servicenamebundle.JobName, servicenamebundle.JobName)
		})

		It("Should set undefined service.name attribute to Pod name", func() {
			assert.VerifyServiceNameAttr(proxyClient, backendExportURL, servicenamebundle.PodWithNoLabelsName, servicenamebundle.PodWithNoLabelsName)
		})

		It("Should enrich service.name attribute when its value is unknown_service", func() {
			assert.VerifyServiceNameAttr(proxyClient, backendExportURL, servicenamebundle.PodWithUnknownServiceName, servicenamebundle.PodWithUnknownServiceName)
		})

		It("Should enrich service.name attribute when its value is following the unknown_service:<process.executable.name> pattern", func() {
			assert.VerifyServiceNameAttr(proxyClient, backendExportURL, servicenamebundle.PodWithUnknownServicePatternName, servicenamebundle.PodWithUnknownServicePatternName)
		})

		It("Should NOT enrich service.name attribute when its value is not following the unknown_service:<process.executable.name> pattern", func() {
			assert.VerifyServiceNameAttr(proxyClient, backendExportURL, servicenamebundle.PodWithInvalidStartForUnknownServicePatternName, servicenamebundle.AttrWithInvalidStartForUnknownServicePattern)
			assert.VerifyServiceNameAttr(proxyClient, backendExportURL, servicenamebundle.PodWithInvalidEndForUnknownServicePatternName, servicenamebundle.AttrWithInvalidEndForUnknownServicePattern)
			assert.VerifyServiceNameAttr(proxyClient, backendExportURL, servicenamebundle.PodWithMissingProcessForUnknownServicePatternName, servicenamebundle.AttrWithMissingProcessForUnknownServicePattern)
		})

		It("Should have no kyma resource attributes", func() {
			assert.VerifyNoKymaAttributes(proxyClient, backendExportURL)
		})
	})
})
