//go:build e2e

package fluentbit

import (
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogsFluentBit, suite.LabelExperimental), Ordered, func() {
	var (
		mockNs           = suite.ID()
		pipelineName     = suite.ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeLogsFluentBit)
		logProducer := loggen.New(mockNs)
		objs = append(objs, backend.K8sObjects()...)
		objs = append(objs, logProducer.K8sObject())
		backendExportURL = backend.ExportURL(suite.ProxyClient)

		// creating a log pipeline explicitly since the testutils.LogPipelineBuilder is not available in the v1beta1 API
		logPipeline := telemetryv1beta1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: pipelineName,
			},
			Spec: telemetryv1beta1.LogPipelineSpec{
				Output: telemetryv1beta1.LogPipelineOutput{
					HTTP: &telemetryv1beta1.LogPipelineHTTPOutput{
						Host: telemetryv1beta1.ValueType{
							Value: backend.Host(),
						},
						Port: strconv.Itoa(int(backend.Port())),
						URI:  "/",
						TLSConfig: telemetryv1beta1.OutputTLS{
							Disabled:                  true,
							SkipCertificateValidation: true,
						},
					},
				},
			},
		}
		objs = append(objs, &logPipeline)

		return objs
	}

	Context("When a logpipeline with HTTP output exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running pipeline", func() {
			assert.LogPipelineHealthy(suite.Ctx, suite.K8sClient, pipelineName)
		})

		It("Should have running log agent", func() {
			assert.DaemonSetReady(suite.Ctx, suite.K8sClient, kitkyma.FluentBitDaemonSetName)
		})

		It("Should have unsupportedMode set to false", func() {
			assert.LogPipelineUnsupportedMode(suite.Ctx, suite.K8sClient, pipelineName, false)
		})

		It("Should have a log producer running", func() {
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Namespace: mockNs, Name: loggen.DefaultName})
		})

		It("Should have produced logs in the backend", func() {
			assert.FluentBitLogsFromPodDelivered(suite.ProxyClient, loggen.DefaultName, backendExportURL)
		})
	})
})
