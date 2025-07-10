package metrics

import (
	"context"
	"net/http"
	"slices"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/metrics/runtime"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestRuntimeInput(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetricsSetC)

	const (
		podNetworkErrorsMetric = "k8s.pod.network.errors"
		podNetworkIOMetric     = "k8s.pod.network.io"

		backendNameA = "backend-a"
		backendNameB = "backend-b"
		backendNameC = "backend-c"

		deploymentName  = "deployment"
		statefulSetName = "statefulset"
		daemonSetName   = "daemonset"
		jobName         = "job"
	)

	var (
		uniquePrefix  = unique.Prefix()
		pipelineNameA = uniquePrefix("a")
		pipelineNameB = uniquePrefix("b")
		pipelineNameC = uniquePrefix("c")

		backendNs = uniquePrefix("backend")
		genNs     = uniquePrefix("gen")
		podNs     = uniquePrefix("pod")

		pvName                  = uniquePrefix()
		pvcName                 = uniquePrefix()
		podMountingPVCName      = uniquePrefix("with-pvc")
		podMountingEmptyDirName = uniquePrefix("with-empty-dir")
	)

	backendA := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName(backendNameA))
	backendB := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName(backendNameB))
	backendC := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName(backendNameC))

	pipelineA := testutils.NewMetricPipelineBuilder().
		WithName(pipelineNameA).
		WithRuntimeInput(true).
		WithRuntimeInputContainerMetrics(true).
		WithRuntimeInputPodMetrics(true).
		WithRuntimeInputNodeMetrics(true).
		WithRuntimeInputVolumeMetrics(true).
		WithRuntimeInputDeploymentMetrics(false).
		WithRuntimeInputStatefulSetMetrics(false).
		WithRuntimeInputDaemonSetMetrics(false).
		WithRuntimeInputJobMetrics(false).
		WithOTLPOutput(testutils.OTLPEndpoint(backendA.Endpoint())).
		Build()
	pipelineB := testutils.NewMetricPipelineBuilder().
		WithName(pipelineNameB).
		WithRuntimeInput(true).
		WithRuntimeInputPodMetrics(false).
		WithRuntimeInputContainerMetrics(false).
		WithRuntimeInputNodeMetrics(false).
		WithRuntimeInputVolumeMetrics(false).
		WithRuntimeInputDeploymentMetrics(true).
		WithRuntimeInputStatefulSetMetrics(true).
		WithRuntimeInputDaemonSetMetrics(true).
		WithRuntimeInputJobMetrics(true).
		WithOTLPOutput(testutils.OTLPEndpoint(backendB.Endpoint())).
		Build()
	pipelineC := testutils.NewMetricPipelineBuilder().
		WithName(pipelineNameC).
		WithRuntimeInput(true).
		WithOTLPOutput(testutils.OTLPEndpoint(backendC.Endpoint())).
		Build()

	prometheusMetricGen := prommetricgen.New(genNs)
	telemetryMetricGen := telemetrygen.PodSpec(telemetrygen.SignalTypeMetrics)

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		kitk8s.NewNamespace(podNs).K8sObject(),
		&pipelineA,
		&pipelineB,
		&pipelineC,
		prometheusMetricGen.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		prometheusMetricGen.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		kitk8s.NewDeployment(deploymentName, genNs).WithPodSpec(telemetryMetricGen).WithLabel("name", deploymentName).K8sObject(),
		kitk8s.NewStatefulSet(statefulSetName, genNs).WithPodSpec(telemetryMetricGen).WithLabel("name", statefulSetName).K8sObject(),
		kitk8s.NewDaemonSet(daemonSetName, genNs).WithPodSpec(telemetryMetricGen).WithLabel("name", daemonSetName).K8sObject(),
		kitk8s.NewJob(jobName, genNs).WithPodSpec(telemetryMetricGen).WithLabel("name", jobName).K8sObject(),
	}
	resources = append(resources, backendA.K8sObjects()...)
	resources = append(resources, backendB.K8sObjects()...)
	resources = append(resources, backendC.K8sObjects()...)
	resources = append(resources, createPodsWithVolume(pvName, pvcName, podMountingPVCName, podMountingEmptyDirName, podNs)...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

	t.Log("Resources should exist and be operational")
	assert.MetricPipelineHealthy(t.Context(), pipelineNameA)
	assert.MetricPipelineHealthy(t.Context(), pipelineNameB)
	assert.MetricPipelineHealthy(t.Context(), pipelineNameC)
	assert.DeploymentReady(t.Context(), kitkyma.MetricGatewayName)
	assert.DaemonSetReady(t.Context(), kitkyma.MetricAgentName)
	assert.BackendReachable(t, backendA)
	assert.BackendReachable(t, backendB)
	assert.BackendReachable(t, backendC)
	assert.DeploymentReady(t.Context(), types.NamespacedName{Name: deploymentName, Namespace: genNs})
	assert.DaemonSetReady(t.Context(), types.NamespacedName{Name: daemonSetName, Namespace: genNs})
	assert.StatefulSetReady(t.Context(), types.NamespacedName{Name: statefulSetName, Namespace: genNs})
	assert.JobReady(t.Context(), types.NamespacedName{Name: jobName, Namespace: genNs})
	assert.PodReady(t.Context(), types.NamespacedName{Name: podMountingPVCName, Namespace: podNs})
	assert.PodReady(t.Context(), types.NamespacedName{Name: podMountingEmptyDirName, Namespace: podNs})
	agentMetricsURL := suite.ProxyClient.ProxyURLForService(kitkyma.MetricAgentMetricsService.Namespace, kitkyma.MetricAgentMetricsService.Name, "metrics", ports.Metrics)
	assert.EmitsOTelCollectorMetrics(t, agentMetricsURL)

	t.Log("Pipeline A should deliver pod, container, volume and node metrics")
	backendContainsMetricsDeliveredForResource(t, backendA, runtime.PodMetricsNames)
	backendContainsMetricsDeliveredForResource(t, backendA, runtime.ContainerMetricsNames)
	backendContainsMetricsDeliveredForResource(t, backendA, runtime.VolumeMetricsNames)
	backendContainsMetricsDeliveredForResource(t, backendA, runtime.NodeMetricsNames)
	backendContainsDesiredResourceAttributes(t, backendA, "k8s.pod.cpu.time", runtime.PodMetricsResourceAttributes)
	backendContainsDesiredResourceAttributes(t, backendA, "container.cpu.time", runtime.ContainerMetricsResourceAttributes)
	backendContainsDesiredResourceAttributes(t, backendA, "k8s.volume.capacity", runtime.VolumeMetricsResourceAttributes)
	backendContainsDesiredResourceAttributes(t, backendA, "k8s.node.cpu.usage", runtime.NodeMetricsResourceAttributes)
	backendContainsDesiredMetricAttributes(t, backendA, podNetworkErrorsMetric, runtime.PodMetricsAttributes[podNetworkErrorsMetric])
	backendContainsDesiredMetricAttributes(t, backendA, podNetworkIOMetric, runtime.PodMetricsAttributes[podNetworkIOMetric])
	assert.BackendDataConsistentlyMatches(t, backendA,
		HaveFlatMetrics(Not(ContainElement(HaveResourceAttributes(HaveKeyWithValue("k8s.volume.type", "emptyDir"))))),
	)

	exportedMetrics := slices.Concat(runtime.PodMetricsNames, runtime.ContainerMetricsNames, runtime.VolumeMetricsNames, runtime.NodeMetricsNames)
	backendConsistsOfMetricsDeliveredForResource(t, backendA, exportedMetrics)

	t.Log("Pipeline B should deliver deployment, daemonset, statefulset and job metrics")
	backendContainsMetricsDeliveredForResource(t, backendB, runtime.DeploymentMetricsNames)
	backendContainsMetricsDeliveredForResource(t, backendB, runtime.DaemonSetMetricsNames)
	backendContainsMetricsDeliveredForResource(t, backendB, runtime.StatefulSetMetricsNames)
	backendContainsMetricsDeliveredForResource(t, backendB, runtime.JobsMetricsNames)
	backendContainsDesiredResourceAttributes(t, backendB, "k8s.deployment.available", runtime.DeploymentResourceAttributes)
	backendContainsDesiredResourceAttributes(t, backendB, "k8s.daemonset.current_scheduled_nodes", runtime.DaemonSetResourceAttributes)
	backendContainsDesiredResourceAttributes(t, backendB, "k8s.statefulset.current_pods", runtime.StatefulSetResourceAttributes)
	backendContainsDesiredResourceAttributes(t, backendB, "k8s.job.active_pods", runtime.JobResourceAttributes)
	expectedMetrics := slices.Concat(runtime.DeploymentMetricsNames, runtime.DaemonSetMetricsNames, runtime.StatefulSetMetricsNames, runtime.JobsMetricsNames)
	backendConsistsOfMetricsDeliveredForResource(t, backendB, expectedMetrics)

	t.Log("Pipeline C should deliver default resource metrics")
	assert.BackendDataEventuallyMatches(t, backendC,
		HaveFlatMetrics(
			ContainElement(SatisfyAll(
				HaveName(BeElementOf(runtime.DefaultMetricsNames)),
				HaveScopeName(Equal(metric.InstrumentationScopeRuntime)),
				HaveScopeVersion(SatisfyAny(
					Equal("main"),
					MatchRegexp("[0-9]+.[0-9]+.[0-9]+"),
				)),
			)),
		))
	assert.MetricsWithScopeAndNamespaceNotDelivered(t, backendC, metric.InstrumentationScopeRuntime, kitkyma.SystemNamespaceName)
	backendConsistsOfMetricsDeliveredForResource(t, backendC, runtime.DefaultMetricsNames)
}

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
func backendContainsMetricsDeliveredForResource(t *testing.T, backend *kitbackend.Backend, resourceMetrics []string) {
	t.Helper()

	Eventually(func(g Gomega) {
		backendURL := backend.ExportURL(suite.ProxyClient)
		resp, err := suite.ProxyClient.Get(backendURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		defer resp.Body.Close()

		g.Expect(resp).To(HaveHTTPBody(
			HaveFlatMetrics(HaveUniqueNamesForRuntimeScope(ContainElements(resourceMetrics))),
		))
	}, 2*periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed(), "Failed to find metrics using ContainElements %v", resourceMetrics)
}

// Check with `ConsistsOf` for metrics present in the backend
func backendConsistsOfMetricsDeliveredForResource(t *testing.T, backend *kitbackend.Backend, resourceMetrics []string) {
	t.Helper()

	Eventually(func(g Gomega) {
		backendURL := backend.ExportURL(suite.ProxyClient)
		resp, err := suite.ProxyClient.Get(backendURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		defer resp.Body.Close()

		g.Expect(resp).To(HaveHTTPBody(
			HaveFlatMetrics(HaveUniqueNamesForRuntimeScope(ConsistOf(resourceMetrics))),
		))
	}, 2*periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed(), "Failed to find metrics using consistsOf %v", resourceMetrics)
}

func backendContainsDesiredResourceAttributes(t *testing.T, backend *kitbackend.Backend, metricName string, resourceAttributes []string) {
	t.Helper()

	Eventually(func(g Gomega) {
		backendURL := backend.ExportURL(suite.ProxyClient)
		resp, err := suite.ProxyClient.Get(backendURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		defer resp.Body.Close()

		g.Expect(resp).To(HaveHTTPBody(HaveFlatMetrics(
			ContainElement(SatisfyAll(
				HaveName(Equal(metricName)),
				HaveResourceAttributes(HaveKeys(ContainElements(resourceAttributes))),
			)),
		)))
	}, 3*periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed(), "Failed to find metric %s with resource attributes %v", metricName, resourceAttributes)
}

func backendContainsDesiredMetricAttributes(t *testing.T, backend *kitbackend.Backend, metricName string, metricAttributes []string) {
	t.Helper()

	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatMetrics(
			ContainElement(SatisfyAll(
				HaveName(Equal(metricName)),
				HaveMetricAttributes(HaveKeys(ConsistOf(metricAttributes))),
			)),
		), "Failed to find metric %s with metric attributes %v", metricName, metricAttributes,
	)
}
