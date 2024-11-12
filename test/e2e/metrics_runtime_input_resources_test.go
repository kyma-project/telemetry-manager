//go:build e2e

package e2e

import (
	"net/http"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/metrics/runtime"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), Label(suite.LabelSetA), Ordered, func() {
	Context("When metric pipelines with runtime resources metrics enabled exist", Ordered, func() {
		var (
			mockNs = suite.ID()

			backendResourceMetricsEnabledNameA  = suite.IDWithSuffix("resource-metrics-a")
			pipelineResourceMetricsEnabledNameA = suite.IDWithSuffix("resource-metrics-a")
			backendResourceMetricsEnabledURLA   string

			backendResourceMetricsEnabledNameB  = suite.IDWithSuffix("resource-metrics-b")
			pipelineResourceMetricsEnabledNameB = suite.IDWithSuffix("resource-metrics-b")
			backendResourceMetricsEnabledURLB   string

			DeploymentName  = suite.IDWithSuffix("deployment")
			StatefulSetName = suite.IDWithSuffix("stateful-set")
			DaemonSetName   = suite.IDWithSuffix("daemon-set")
			JobName         = suite.IDWithSuffix("job")

			pvName                  = suite.IDWithSuffix("pv")
			pvcName                 = suite.IDWithSuffix("pvc")
			podMountingPVCName      = suite.IDWithSuffix("pod-with-pvc")
			podMountingEmptyDirName = suite.IDWithSuffix("pod-with-emptydir")
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

			backendResourceMetricsEnabledA := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendResourceMetricsEnabledNameA))
			objs = append(objs, backendResourceMetricsEnabledA.K8sObjects()...)
			backendResourceMetricsEnabledURLA = backendResourceMetricsEnabledA.ExportURL(proxyClient)

			pipelineResourceMetricsEnabledA := testutils.NewMetricPipelineBuilder().
				WithName(pipelineResourceMetricsEnabledNameA).
				WithRuntimeInput(true).
				WithRuntimeInputContainerMetrics(true).
				WithRuntimeInputPodMetrics(true).
				WithRuntimeInputNodeMetrics(true).
				WithRuntimeInputVolumeMetrics(true).
				WithRuntimeInputDeploymentMetrics(false).
				WithRuntimeInputStatefulSetMetrics(false).
				WithRuntimeInputDaemonSetMetrics(false).
				WithRuntimeInputJobMetrics(false).
				WithOTLPOutput(testutils.OTLPEndpoint(backendResourceMetricsEnabledA.Endpoint())).
				Build()
			objs = append(objs, &pipelineResourceMetricsEnabledA)

			backendResourceMetricsEnabledB := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendResourceMetricsEnabledNameB))
			objs = append(objs, backendResourceMetricsEnabledB.K8sObjects()...)
			backendResourceMetricsEnabledURLB = backendResourceMetricsEnabledB.ExportURL(proxyClient)

			pipelineResourceMetricsEnabledB := testutils.NewMetricPipelineBuilder().
				WithName(pipelineResourceMetricsEnabledNameB).
				WithRuntimeInput(true).
				WithRuntimeInputPodMetrics(false).
				WithRuntimeInputContainerMetrics(false).
				WithRuntimeInputNodeMetrics(false).
				WithRuntimeInputVolumeMetrics(false).
				WithRuntimeInputDeploymentMetrics(true).
				WithRuntimeInputStatefulSetMetrics(true).
				WithRuntimeInputDaemonSetMetrics(true).
				WithRuntimeInputJobMetrics(true).
				WithOTLPOutput(testutils.OTLPEndpoint(backendResourceMetricsEnabledB.Endpoint())).
				Build()
			objs = append(objs, &pipelineResourceMetricsEnabledB)

			metricProducer := prommetricgen.New(mockNs)

			objs = append(objs, []client.Object{
				metricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
				metricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
			}...)

			podSpec := telemetrygen.PodSpec(telemetrygen.SignalTypeMetrics)

			objs = append(objs, []client.Object{
				kitk8s.NewDeployment(DeploymentName, mockNs).WithPodSpec(podSpec).WithLabel("name", DeploymentName).K8sObject(),
				kitk8s.NewStatefulSet(StatefulSetName, mockNs).WithPodSpec(podSpec).WithLabel("name", StatefulSetName).K8sObject(),
				kitk8s.NewDaemonSet(DaemonSetName, mockNs).WithPodSpec(podSpec).WithLabel("name", DaemonSetName).K8sObject(),
				kitk8s.NewJob(JobName, mockNs).WithPodSpec(podSpec).WithLabel("name", JobName).K8sObject(),
			}...)

			objs = append(objs, createPodsWithVolume(pvName, pvcName, podMountingPVCName, podMountingEmptyDirName, mockNs)...)

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
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineResourceMetricsEnabledNameA)
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineResourceMetricsEnabledNameB)
		})

		It("Ensures the metric gateway deployment is ready", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Ensures the metric agent daemonset is ready", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		It("Should have metrics backends running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backendResourceMetricsEnabledNameA, Namespace: mockNs})
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backendResourceMetricsEnabledNameB, Namespace: mockNs})
			assert.ServiceReady(ctx, k8sClient, types.NamespacedName{Name: backendResourceMetricsEnabledNameA, Namespace: mockNs})
			assert.ServiceReady(ctx, k8sClient, types.NamespacedName{Name: backendResourceMetricsEnabledNameB, Namespace: mockNs})

		})

		It("should have workloads created properly", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: DeploymentName, Namespace: mockNs})
			assert.DaemonSetReady(ctx, k8sClient, types.NamespacedName{Name: DaemonSetName, Namespace: mockNs})
			assert.StatefulSetReady(ctx, k8sClient, types.NamespacedName{Name: StatefulSetName, Namespace: mockNs})
			assert.JobReady(ctx, k8sClient, types.NamespacedName{Name: JobName, Namespace: mockNs})
		})

		It("Should have pods mounting volumes running", func() {
			assert.PodReady(ctx, k8sClient, types.NamespacedName{Name: podMountingPVCName, Namespace: mockNs})
			assert.PodReady(ctx, k8sClient, types.NamespacedName{Name: podMountingEmptyDirName, Namespace: mockNs})
		})

		Context("Pipeline A should deliver pods, container, volume and nodes metrics", func() {
			It("Should deliver pod metrics with expected resource attributes and metric attributes", func() {
				backendContainsMetricsDeliveredForResource(proxyClient, backendResourceMetricsEnabledURLA, runtime.PodMetricsNames)
				backendContainsDesiredResourceAttributes(proxyClient, backendResourceMetricsEnabledURLA, "k8s.pod.cpu.time", runtime.PodMetricsResourceAttributes)

				podNetworkErrorsMetric := "k8s.pod.network.errors"
				backendContainsDesiredMetricAttributes(proxyClient, backendResourceMetricsEnabledURLA, podNetworkErrorsMetric, runtime.PodMetricsAttributes[podNetworkErrorsMetric])

				podNetworkIOMetric := "k8s.pod.network.io"
				backendContainsDesiredMetricAttributes(proxyClient, backendResourceMetricsEnabledURLA, podNetworkIOMetric, runtime.PodMetricsAttributes[podNetworkIOMetric])

			})

			It("Should deliver container metrics with expected resource attributes", func() {
				backendContainsMetricsDeliveredForResource(proxyClient, backendResourceMetricsEnabledURLA, runtime.ContainerMetricsNames)
				backendContainsDesiredResourceAttributes(proxyClient, backendResourceMetricsEnabledURLA, "container.cpu.time", runtime.ContainerMetricsResourceAttributes)
			})

			It("Should deliver volume metrics with expected resource attributes", func() {
				backendContainsMetricsDeliveredForResource(proxyClient, backendResourceMetricsEnabledURLA, runtime.VolumeMetricsNames)
				backendContainsDesiredResourceAttributes(proxyClient, backendResourceMetricsEnabledURLA, "k8s.volume.capacity", runtime.VolumeMetricsResourceAttributes)
			})

			It("Should filter volume metrics only for PVC volumes", func() {
				Consistently(func(g Gomega) {
					resp, err := proxyClient.Get(backendResourceMetricsEnabledURLA)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

					g.Expect(resp).To(HaveHTTPBody(
						HaveFlatMetrics(Not(ContainElement(HaveResourceAttributes(HaveKeyWithValue("k8s.volume.type", "emptyDir"))))),
					))
				}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
			})

			It("Should deliver node metrics with expected resource attributes", func() {
				backendContainsMetricsDeliveredForResource(proxyClient, backendResourceMetricsEnabledURLA, runtime.NodeMetricsNames)
				backendContainsDesiredResourceAttributes(proxyClient, backendResourceMetricsEnabledURLA, "k8s.node.cpu.usage", runtime.NodeMetricsResourceAttributes)
			})

			It("Should have exactly metrics only for pods, containers, volumes and nodes delivered", func() {
				exportedMetrics := slices.Concat(runtime.PodMetricsNames, runtime.ContainerMetricsNames, runtime.VolumeMetricsNames, runtime.NodeMetricsNames)
				backendConsistsOfMetricsDeliveredForResource(proxyClient, backendResourceMetricsEnabledURLA, exportedMetrics)
			})
		})

		Context("Pipeline B should deliver deployment, daemonset, statefulset and job metrics", Ordered, func() {
			It("should deliver deployment metrics with expected resource attributes", func() {
				backendContainsMetricsDeliveredForResource(proxyClient, backendResourceMetricsEnabledURLB, runtime.DeploymentMetricsNames)
				backendContainsDesiredResourceAttributes(proxyClient, backendResourceMetricsEnabledURLB, "k8s.deployment.available", runtime.DeploymentResourceAttributes)
			})

			It("should deliver daemonset metrics with expected resource attributes", func() {
				backendContainsMetricsDeliveredForResource(proxyClient, backendResourceMetricsEnabledURLB, runtime.DaemonSetMetricsNames)
				backendContainsDesiredResourceAttributes(proxyClient, backendResourceMetricsEnabledURLB, "k8s.daemonset.current_scheduled_nodes", runtime.DaemonSetResourceAttributes)
			})

			It("should deliver statefulset metrics with expected resource attributes", func() {
				backendContainsMetricsDeliveredForResource(proxyClient, backendResourceMetricsEnabledURLB, runtime.StatefulSetMetricsNames)
				backendContainsDesiredResourceAttributes(proxyClient, backendResourceMetricsEnabledURLB, "k8s.statefulset.current_pods", runtime.StatefulSetResourceAttributes)
			})

			It("should deliver job metrics with expected resource attributes", func() {
				backendContainsMetricsDeliveredForResource(proxyClient, backendResourceMetricsEnabledURLB, runtime.JobsMetricsNames)
				backendContainsDesiredResourceAttributes(proxyClient, backendResourceMetricsEnabledURLB, "k8s.job.active_pods", runtime.JobResourceAttributes)
			})

			It("should have exactly metrics only for deployment, daemonset, statefuleset, job delivered", func() {
				expectedMetrics := slices.Concat(runtime.DeploymentMetricsNames, runtime.DaemonSetMetricsNames, runtime.StatefulSetMetricsNames, runtime.JobsMetricsNames)
				backendConsistsOfMetricsDeliveredForResource(proxyClient, backendResourceMetricsEnabledURLB, expectedMetrics)
			})
		})
	})

})

func createPodsWithVolume(pvName, pvcName, podMountingPVCName, podMountingEmptyDirName, namespace string) []client.Object {
	var objs []client.Object

	storageClassName := "test-storage"
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: pvName, Namespace: namespace},
		Spec: corev1.PersistentVolumeSpec{
			Capacity:         corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("500Mi")},
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: storageClassName,
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				Local: &corev1.LocalVolumeSource{
					Path: "/var",
				},
			},
			NodeAffinity: &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "kubernetes.io/os",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"linux"},
								},
							},
						},
					},
				},
			},
		},
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: pvcName, Namespace: namespace},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClassName,
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("200Mi")},
			},
		},
	}

	podMountingPVC := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podMountingPVCName, Namespace: namespace},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: "pvc-volume",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "nginx:latest",
					VolumeMounts: []corev1.VolumeMount{
						{
							MountPath: "/mnt",
							Name:      "pvc-volume",
						},
					},
				},
			},
		},
	}

	// create a pod mounting an emptyDir volume to ensure only metrics for PVC volumes are delivered
	podMountingEmptyDir := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podMountingEmptyDirName, Namespace: namespace},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: "emptydir-volume",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "nginx:latest",
					VolumeMounts: []corev1.VolumeMount{
						{
							MountPath: "/mnt",
							Name:      "emptydir-volume",
						},
					},
				},
			},
		},
	}

	objs = append(objs, pv, pvc, podMountingPVC, podMountingEmptyDir)

	return objs
}

// Check for `ContainElements` for metrics present in the backend
func backendContainsMetricsDeliveredForResource(proxyClient *apiserverproxy.Client, backendExportURL string, resourceMetrics []string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(backendExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		defer resp.Body.Close()

		g.Expect(resp).To(HaveHTTPBody(
			HaveFlatMetrics(HaveUniqueNamesForRuntimeScope(ContainElements(resourceMetrics))),
		))
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed(), "Failed to find metrics using ContainElements %v", resourceMetrics)
}

func backendContainsDesiredResourceAttributes(proxyClient *apiserverproxy.Client, backendExportURL string, metricName string, resourceAttributes []string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(backendExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		defer resp.Body.Close()

		g.Expect(resp).To(HaveHTTPBody(HaveFlatMetrics(
			ContainElement(SatisfyAll(
				HaveName(Equal(metricName)),
				HaveResourceAttributes(HaveKeys(ConsistOf(resourceAttributes))),
			)),
		)))
	}, 3*periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed(), "Failed to find metric %s with resource attributes %v", metricName, resourceAttributes)
}

func backendContainsDesiredMetricAttributes(proxyClient *apiserverproxy.Client, backendExportURL string, metricName string, metricAttributes []string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(backendExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		defer resp.Body.Close()

		g.Expect(resp).To(HaveHTTPBody(HaveFlatMetrics(
			ContainElement(SatisfyAll(
				HaveName(Equal(metricName)),
				HaveMetricAttributes(HaveKeys(ConsistOf(metricAttributes))),
			)),
		)))
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed(), "Failed to find metric %s with metric attributes %v", metricName, metricAttributes)
}

// Check with `ConsistsOf` for metrics present in the backend
func backendConsistsOfMetricsDeliveredForResource(proxyClient *apiserverproxy.Client, backendExportURL string, resourceMetrics []string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(backendExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		defer resp.Body.Close()

		g.Expect(resp).To(HaveHTTPBody(
			HaveFlatMetrics(HaveUniqueNamesForRuntimeScope(ConsistOf(resourceMetrics))),
		))
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed(), "Failed to find metrics using consistsOf %v", resourceMetrics)
}
