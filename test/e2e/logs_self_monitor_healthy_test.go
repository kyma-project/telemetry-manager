//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/prometheus"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelSelfMonitoringLogsHealthy), Ordered, func() {
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

		logPipeline := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
			Build()
		objs = append(objs, &logPipeline)

		return objs
	}

	Context("Before deploying a logpipeline", func() {
		It("Should have a healthy webhook", func() {
			assert.WebhookHealthy(ctx, k8sClient)
		})
	})

	Context("When a logpipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running logpipeline", func() {
			assert.LogPipelineHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should have running log agent", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.FluentBitDaemonSet)
		})

		It("Should have a running self-monitor", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.SelfMonitorName)
		})

		It("Should have a log backend running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: backend.DefaultName})
		})

		It("Should have a log producer running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: loggen.DefaultName})
		})

		It("Should have produced logs in the backend", func() {
			assert.LogsDelivered(proxyClient, loggen.DefaultName, backendExportURL)
		})

		It("Should have TypeFlowHealthy condition set to True", func() {
			Eventually(func(g Gomega) {
				var pipeline telemetryv1alpha1.LogPipeline
				key := types.NamespacedName{Name: pipelineName}
				g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypeFlowHealthy)).To(BeTrueBecause("Flow not healthy"))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		Context("Metric instrumentation", Ordered, func() {
			It("Ensures that controller_runtime_webhook_requests_total is increased", func() {
				// Pushing metrics to the metric gateway triggers an alert.
				// It makes the self-monitor call the webhook, which in turn increases the counter.
				assert.ManagerEmitsMetric(proxyClient,
					Equal("controller_runtime_webhook_requests_total"),
					SatisfyAll(
						WithLabels(HaveKeyWithValue("webhook", "/api/v2/alerts")),
						WithValue(BeNumerically(">", 0)),
					))
			})

			It("Ensures that telemetry_self_monitor_prober_requests_total is emitted", func() {
				assert.ManagerEmitsMetric(proxyClient,
					Equal("telemetry_self_monitor_prober_requests_total"),
					WithValue(BeNumerically(">", 0)),
				)
			})
		})
	})
})
