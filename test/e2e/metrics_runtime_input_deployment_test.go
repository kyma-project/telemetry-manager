//go:build e2e

package e2e

import (
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/metrics/runtime"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"slices"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), Label(suite.LabelSetA), Ordered, func() {
	Context("When metric pipelines with deployment metrics enabled exist", Ordered, func() {
		var (
			mockNs     = suite.IDWithSuffix("resource-metrics")
			workloadNs = suite.IDWithSuffix("workloads")

			backendWorkloadMetricsEnabledName  = suite.IDWithSuffix("resource-metrics-enabled")
			pipelineWorkloadMetricsEnabledName = suite.IDWithSuffix("resource-metrics-enabled")
			backendWorkloadMetricsEnabledURL   string

			backendWorkloadMetricsDisabledName  = suite.IDWithSuffix("resource-metrics-disabled")
			pipelineWorkloadMetricsDisabledName = suite.IDWithSuffix("resource-metrics-disabled")
			backendWorkloadMetricsDisabledURL   string

			DeploymentName  = "deployment"
			StatefulSetName = "stateful-set"
			DaemonSetName   = "daemon-set"
			JobName         = "job"
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())
			objs = append(objs, kitk8s.NewNamespace(workloadNs).K8sObject())

			backendWorkloadMetricsEnabled := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendWorkloadMetricsEnabledName))
			objs = append(objs, backendWorkloadMetricsEnabled.K8sObjects()...)

			backendWorkloadMetricsDisabled := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendWorkloadMetricsDisabledName))
			objs = append(objs, backendWorkloadMetricsDisabled.K8sObjects()...)

			backendWorkloadMetricsEnabledURL = backendWorkloadMetricsEnabled.ExportURL(proxyClient)
			backendWorkloadMetricsDisabledURL = backendWorkloadMetricsDisabled.ExportURL(proxyClient)

			pipelineResourcesMetricsEnabledA := testutils.NewMetricPipelineBuilder().
				WithName(pipelineWorkloadMetricsEnabledName).
				WithRuntimeInput(true).
				WithRuntimeInputPodMetrics(false).
				WithRuntimeInputContainerMetrics(false).
				WithRuntimeInputDeploymentMetrics(true).
				WithRuntimeInputStatefulSetMetrics(true).
				WithRuntimeInputDaemonSetMetrics(true).
				WithRuntimeInputJobMetrics(true).
				WithOTLPOutput(testutils.OTLPEndpoint(backendWorkloadMetricsEnabled.Endpoint())).
				Build()
			objs = append(objs, &pipelineResourcesMetricsEnabledA)

			pipelineWorkloadMetricsDisabled := testutils.NewMetricPipelineBuilder().
				WithName(pipelineWorkloadMetricsDisabledName).
				WithRuntimeInput(true).
				WithOTLPOutput(testutils.OTLPEndpoint(backendWorkloadMetricsDisabled.Endpoint())).
				Build()
			objs = append(objs, &pipelineWorkloadMetricsDisabled)

			podSpec := telemetrygen.PodSpec(telemetrygen.SignalTypeMetrics)

			objs = append(objs, []client.Object{
				kitk8s.NewDeployment(DeploymentName, workloadNs).WithPodSpec(podSpec).K8sObject(),
				kitk8s.NewStatefulSet(StatefulSetName, workloadNs).WithPodSpec(podSpec).K8sObject(),
				kitk8s.NewDaemonSet(DaemonSetName, workloadNs).WithPodSpec(podSpec).K8sObject(),
				kitk8s.NewJob(JobName, workloadNs).WithPodSpec(podSpec).K8sObject(),
			}...)

			return objs
		}
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have healthy pipelines", func() {
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineWorkloadMetricsEnabledName)
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineWorkloadMetricsDisabledName)
		})

		It("Ensures the metric gateway deployment is ready", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Ensures the metric agent daemonset is ready", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		It("Should have metrics backends running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backendWorkloadMetricsEnabledName, Namespace: mockNs})
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backendWorkloadMetricsDisabledName, Namespace: mockNs})
		})

		It("should have workloads created properly", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: DeploymentName, Namespace: workloadNs})
			assert.DaemonSetReady(ctx, k8sClient, types.NamespacedName{Name: DaemonSetName, Namespace: workloadNs})
			assert.StatefulSetReady(ctx, k8sClient, types.NamespacedName{Name: StatefulSetName, Namespace: workloadNs})
			assert.JobReady(ctx, k8sClient, types.NamespacedName{Name: JobName, Namespace: workloadNs})
		})

		It("should have metrics for deployments delivered", func() {
			assert.BackendContainsMetricsDeliveredForResource(proxyClient, backendWorkloadMetricsEnabledURL, runtime.DeploymentMetricsNames)
		})

		It("should have metrics for daemonset delivered", func() {
			assert.BackendContainsMetricsDeliveredForResource(proxyClient, backendWorkloadMetricsEnabledURL, runtime.DaemonSetMetricsNames)
		})

		It("should have metrics for statefulset delivered", func() {
			assert.BackendContainsMetricsDeliveredForResource(proxyClient, backendWorkloadMetricsEnabledURL, runtime.StatefulSetMetricsNames)
		})

		It("should have metrics for job delivered", func() {
			assert.BackendContainsMetricsDeliveredForResource(proxyClient, backendWorkloadMetricsEnabledURL, runtime.JobsMetricsNames)
		})

		It("should have exactly metrics only for deployment, daemonset, statefuleset, job delivered", func() {
			expectedMetrics := slices.Concat(runtime.DeploymentMetricsNames, runtime.DaemonSetMetricsNames, runtime.StatefulSetMetricsNames, runtime.JobsMetricsNames)
			assert.BackendConsistsMetricsDeliveredForResource(proxyClient, backendWorkloadMetricsEnabledURL, expectedMetrics)
		})

		fmt.Printf("Metrics backends URLs Enabled: %s, Disabled: %s ", backendWorkloadMetricsEnabledURL, backendWorkloadMetricsDisabledURL)
	})

})
