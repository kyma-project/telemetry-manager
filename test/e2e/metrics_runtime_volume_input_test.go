//go:build e2e

package e2e

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/metrics/runtime"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), Ordered, func() {
	Context("When a metric pipeline with ONLY volume metrics enabled exists", Ordered, func() {
		var (
			mockNs = suite.ID()

			backendOnlyVolumeMetricsEnabledName  = suite.ID()
			pipelineOnlyVolumeMetricsEnabledName = suite.ID()
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
