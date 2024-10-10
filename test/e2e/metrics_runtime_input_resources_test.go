//go:build e2e

package e2e

import (
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/metrics/runtime"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), Ordered, func() {
	Context("When metric pipelines with container, pod and node metrics enabled exist", Ordered, func() {
		var (
			mockNs = suite.IDWithSuffix("container-pod-node-metrics")

			backendOnlyContainerMetricsEnabledName  = suite.IDWithSuffix("container-metrics")
			pipelineOnlyContainerMetricsEnabledName = suite.IDWithSuffix("container-metrics")
			backendOnlyContainerMetricsEnabledURL   string

			backendOnlyPodMetricsEnabledName  = suite.IDWithSuffix("pod-metrics")
			pipelineOnlyPodMetricsEnabledName = suite.IDWithSuffix("pod-metrics")
			backendOnlyPodMetricsEnabledURL   string

			backendOnlyNodeMetricsEnabledName  = suite.IDWithSuffix("node-metrics")
			pipelineOnlyNodeMetricsEnabledName = suite.IDWithSuffix("node-metrics")
			backendOnlyNodeMetricsEnabledURL   string
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

			backendOnlyContainerMetricsEnabled := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendOnlyContainerMetricsEnabledName))
			objs = append(objs, backendOnlyContainerMetricsEnabled.K8sObjects()...)
			backendOnlyContainerMetricsEnabledURL = backendOnlyContainerMetricsEnabled.ExportURL(proxyClient)

			pipelineOnlyContainerMetricsEnabled := testutils.NewMetricPipelineBuilder().
				WithName(pipelineOnlyContainerMetricsEnabledName).
				WithRuntimeInput(true).
				WithRuntimeInputContainerMetrics(true).
				WithRuntimeInputPodMetrics(false).
				WithRuntimeInputNodeMetrics(false).
				WithRuntimeInputVolumeMetrics(false).
				WithOTLPOutput(testutils.OTLPEndpoint(backendOnlyContainerMetricsEnabled.Endpoint())).
				Build()
			objs = append(objs, &pipelineOnlyContainerMetricsEnabled)

			backendOnlyPodMetricsEnabled := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendOnlyPodMetricsEnabledName))
			objs = append(objs, backendOnlyPodMetricsEnabled.K8sObjects()...)
			backendOnlyPodMetricsEnabledURL = backendOnlyPodMetricsEnabled.ExportURL(proxyClient)

			pipelineOnlyPodMetricsEnabled := testutils.NewMetricPipelineBuilder().
				WithName(pipelineOnlyPodMetricsEnabledName).
				WithRuntimeInput(true).
				WithRuntimeInputPodMetrics(true).
				WithRuntimeInputContainerMetrics(false).
				WithRuntimeInputNodeMetrics(false).
				WithRuntimeInputVolumeMetrics(false).
				WithOTLPOutput(testutils.OTLPEndpoint(backendOnlyPodMetricsEnabled.Endpoint())).
				Build()
			objs = append(objs, &pipelineOnlyPodMetricsEnabled)

			backendOnlyNodeMetricsEnabled := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendOnlyNodeMetricsEnabledName))
			objs = append(objs, backendOnlyNodeMetricsEnabled.K8sObjects()...)
			backendOnlyNodeMetricsEnabledURL = backendOnlyNodeMetricsEnabled.ExportURL(proxyClient)

			pipelineOnlyNodeMetricsEnabled := testutils.NewMetricPipelineBuilder().
				WithName(pipelineOnlyNodeMetricsEnabledName).
				WithRuntimeInput(true).
				WithRuntimeInputNodeMetrics(true).
				WithRuntimeInputPodMetrics(false).
				WithRuntimeInputContainerMetrics(false).
				WithRuntimeInputVolumeMetrics(false).
				WithOTLPOutput(testutils.OTLPEndpoint(backendOnlyNodeMetricsEnabled.Endpoint())).
				Build()
			objs = append(objs, &pipelineOnlyNodeMetricsEnabled)

			metricProducer := prommetricgen.New(mockNs)

			objs = append(objs, []client.Object{
				metricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
				metricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
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
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineOnlyContainerMetricsEnabledName)
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineOnlyPodMetricsEnabledName)
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineOnlyNodeMetricsEnabledName)
		})

		It("Ensures the metric gateway deployment is ready", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Ensures the metric agent daemonset is ready", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		It("Should have metrics backends running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backendOnlyContainerMetricsEnabledName, Namespace: mockNs})
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backendOnlyPodMetricsEnabledName, Namespace: mockNs})
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backendOnlyNodeMetricsEnabledName, Namespace: mockNs})

		})

		Context("Runtime container metrics", func() {
			It("Should deliver ONLY runtime container metrics to container-metrics backend", func() {
				Eventually(func(g Gomega) {
					resp, err := proxyClient.Get(backendOnlyContainerMetricsEnabledURL)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

					g.Expect(resp).To(HaveHTTPBody(
						HaveFlatMetrics(HaveUniqueNamesForRuntimeScope(ConsistOf(runtime.ContainerMetricsNames))),
					))
				}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
			})

			It("Should have expected resource attributes in runtime container metrics", func() {
				Eventually(func(g Gomega) {
					resp, err := proxyClient.Get(backendOnlyContainerMetricsEnabledURL)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

					g.Expect(resp).To(HaveHTTPBody(
						HaveFlatMetrics(ContainElement(HaveResourceAttributes(HaveKeys(ConsistOf(runtime.ContainerMetricsResourceAttributes))))),
					))
				}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
			})
		})

		Context("Runtime pod metrics", func() {
			It("Should deliver ONLY runtime pod metrics to pod-metrics backend", func() {
				Eventually(func(g Gomega) {
					resp, err := proxyClient.Get(backendOnlyPodMetricsEnabledURL)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

					g.Expect(resp).To(HaveHTTPBody(
						HaveFlatMetrics(HaveUniqueNamesForRuntimeScope(ConsistOf(runtime.PodMetricsNames))),
					))
				}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
			})

			It("Should have expected resource attributes in runtime pod metrics", func() {
				Eventually(func(g Gomega) {
					resp, err := proxyClient.Get(backendOnlyPodMetricsEnabledURL)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

					g.Expect(resp).To(HaveHTTPBody(
						HaveFlatMetrics(ContainElement(HaveResourceAttributes(HaveKeys(ConsistOf(runtime.PodMetricsResourceAttributes))))),
					))
				}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
			})

			It("Should have expected metric attributes in runtime pod metrics", func() {
				Eventually(func(g Gomega) {
					resp, err := proxyClient.Get(backendOnlyPodMetricsEnabledURL)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

					bodyContent, err := io.ReadAll(resp.Body)
					defer resp.Body.Close()
					g.Expect(err).NotTo(HaveOccurred())

					podNetworkErrorsMetric := "k8s.pod.network.errors"
					g.Expect(bodyContent).To(HaveFlatMetrics(
						ContainElement(SatisfyAll(
							HaveName(Equal(podNetworkErrorsMetric)),
							HaveMetricAttributes(HaveKeys(ConsistOf(runtime.PodMetricsAttributes[podNetworkErrorsMetric]))),
						)),
					))

					podNetworkIOMetric := "k8s.pod.network.io"
					g.Expect(bodyContent).To(HaveFlatMetrics(
						ContainElement(SatisfyAll(
							HaveName(Equal(podNetworkIOMetric)),
							HaveMetricAttributes(HaveKeys(ConsistOf(runtime.PodMetricsAttributes[podNetworkIOMetric]))),
						)),
					))
				}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
			})
		})

		Context("Runtime node metrics", func() {
			It("Should deliver ONLY runtime node metrics to node-metrics backend", func() {
				Eventually(func(g Gomega) {
					resp, err := proxyClient.Get(backendOnlyNodeMetricsEnabledURL)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

					g.Expect(resp).To(HaveHTTPBody(
						HaveFlatMetrics(HaveUniqueNamesForRuntimeScope(ConsistOf(runtime.NodeMetricsNames))),
					))
				}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
			})

			It("Should have expected resource attributes in runtime node metrics", func() {
				Eventually(func(g Gomega) {
					resp, err := proxyClient.Get(backendOnlyNodeMetricsEnabledURL)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

					g.Expect(resp).To(HaveHTTPBody(
						HaveFlatMetrics(ContainElement(HaveResourceAttributes(HaveKeys(ConsistOf(runtime.NodeMetricsResourceAttributes))))),
					))
				}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
			})
		})
	})

	Context("When a metric pipeline with volume metrics enabled exists", Ordered, func() {
		var (
			mockNs = suite.IDWithSuffix("volume-metrics")

			backendOnlyVolumeMetricsEnabledName  = suite.IDWithSuffix("volume-metrics")
			pipelineOnlyVolumeMetricsEnabledName = suite.IDWithSuffix("volume-metrics")
			backendOnlyVolumeMetricsEnabledURL   string

			pvName                  = suite.IDWithSuffix("pv")
			pvcName                 = suite.IDWithSuffix("pvc")
			podMountingPVCName      = suite.IDWithSuffix("pod-with-pvc")
			podMountingEmptyDirName = suite.IDWithSuffix("pod-with-emptydir")
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

			backendOnlyVolumeMetricsEnabled := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendOnlyVolumeMetricsEnabledName))
			objs = append(objs, backendOnlyVolumeMetricsEnabled.K8sObjects()...)
			backendOnlyVolumeMetricsEnabledURL = backendOnlyVolumeMetricsEnabled.ExportURL(proxyClient)

			pipelineOnlyVolumeMetricsEnabled := testutils.NewMetricPipelineBuilder().
				WithName(pipelineOnlyVolumeMetricsEnabledName).
				WithRuntimeInput(true).
				WithRuntimeInputContainerMetrics(false).
				WithRuntimeInputPodMetrics(false).
				WithRuntimeInputNodeMetrics(false).
				WithRuntimeInputVolumeMetrics(true).
				WithOTLPOutput(testutils.OTLPEndpoint(backendOnlyVolumeMetricsEnabled.Endpoint())).
				Build()
			objs = append(objs, &pipelineOnlyVolumeMetricsEnabled)

			storageClassName := "test-storage"

			pv := &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{Name: pvName, Namespace: mockNs},
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
				ObjectMeta: metav1.ObjectMeta{Name: pvcName, Namespace: mockNs},
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: &storageClassName,
					AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("200Mi")},
					},
				},
			}

			podMountingPVC := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: podMountingPVCName, Namespace: mockNs},
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
				ObjectMeta: metav1.ObjectMeta{Name: podMountingEmptyDirName, Namespace: mockNs},
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

		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have healthy pipelines", func() {
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineOnlyVolumeMetricsEnabledName)
		})

		It("Ensures the metric gateway deployment is ready", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Ensures the metric agent daemonset is ready", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		It("Should have metrics backends running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backendOnlyVolumeMetricsEnabledName, Namespace: mockNs})
		})

		Context("Runtime volume metrics", func() {
			It("Should deliver ONLY runtime volume metrics to volume-metrics backend", func() {
				Eventually(func(g Gomega) {
					resp, err := proxyClient.Get(backendOnlyVolumeMetricsEnabledURL)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

					g.Expect(resp).To(HaveHTTPBody(
						HaveFlatMetrics(HaveUniqueNamesForRuntimeScope(ConsistOf(runtime.VolumeMetricsNames))),
					))
				}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
			})

			It("Should have expected resource attributes in runtime volume metrics", func() {
				Eventually(func(g Gomega) {
					resp, err := proxyClient.Get(backendOnlyVolumeMetricsEnabledURL)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

					g.Expect(resp).To(HaveHTTPBody(
						HaveFlatMetrics(ContainElement(HaveResourceAttributes(HaveKeys(ConsistOf(runtime.VolumeMetricsResourceAttributes))))),
					))
				}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
			})

			It("Should filter volume metrics only for PVC volumes", func() {
				Consistently(func(g Gomega) {
					resp, err := proxyClient.Get(backendOnlyVolumeMetricsEnabledURL)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

					g.Expect(resp).To(HaveHTTPBody(
						HaveFlatMetrics(Not(ContainElement(HaveResourceAttributes(HaveKeyWithValue("k8s.volume.type", "emptyDir"))))),
					))
				}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
			})
		})
	})
})
