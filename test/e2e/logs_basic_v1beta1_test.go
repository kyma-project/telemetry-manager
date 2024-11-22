//go:build e2e

package e2e

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

var _ = Describe(suite.ID(), Label(suite.LabelLogs, suite.LabelExperimental), Ordered, func() {
	var (
		mockNs           = suite.ID()
		pipelineName     = suite.ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeLogs)
		logProducer := loggen.New(mockNs)
		objs = append(objs, backend.K8sObjects()...)
		objs = append(objs, logProducer.K8sObject())
		backendExportURL = backend.ExportURL(proxyClient)

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
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running pipeline", func() {
			assert.LogPipelineHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should have running log agent", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.FluentBitDaemonSetName)
		})

		It("Should have unsupportedMode set to false", func() {
			assert.LogPipelineUnsupportedMode(ctx, k8sClient, pipelineName, false)
		})

		It("Should have a log producer running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: loggen.DefaultName})
		})

		It("Should have produced logs in the backend", func() {
			assert.LogsDelivered(proxyClient, loggen.DefaultName, backendExportURL)
		})
	})

	Context("When a logpipeline with OTLP output exists", Ordered, func() {
		// TODO: Copy from v1alpha1
	})
})
