package otelcollector

import (
	"maps"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/kyma-project/telemetry-manager/internal/config"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
)

// WorkloadMetadata contains labels and annotations for workload resources
type WorkloadMetadata struct {
	ResourceLabels      map[string]string
	ResourceAnnotations map[string]string
	PodLabels           map[string]string
	PodAnnotations      map[string]string
}

// MakeWorkloadMetadata creates metadata for workloads (Deployment or DaemonSet).
// Resource labels only contain additional labels from globals; default labels are applied by the Labeler.
// Pod labels need default labels explicitly since the Labeler only sets top-level object labels.
func MakeWorkloadMetadata(globals *config.Global, baseName string, componentType string, extraPodLabels map[string]string, extraPodAnnotations map[string]string) WorkloadMetadata {
	// Resource labels: only additional labels from globals; default labels are applied by the Labeler
	resourceLabels := make(map[string]string)
	maps.Copy(resourceLabels, globals.AdditionalWorkloadLabels())

	// Pod labels: need default labels explicitly since the Labeler only sets top-level object labels
	podLabels := make(map[string]string)
	maps.Copy(podLabels, commonresources.DefaultLabels(baseName, componentType))
	maps.Copy(podLabels, globals.AdditionalWorkloadLabels())
	maps.Copy(podLabels, extraPodLabels)

	// Resource annotations: only additional annotations from globals
	resourceAnnotations := make(map[string]string)
	maps.Copy(resourceAnnotations, globals.AdditionalWorkloadAnnotations())

	// Pod annotations: additional annotations from globals + workload-specific annotations
	podAnnotations := make(map[string]string)
	maps.Copy(podAnnotations, globals.AdditionalWorkloadAnnotations())
	maps.Copy(podAnnotations, extraPodAnnotations)

	return WorkloadMetadata{
		ResourceLabels:      resourceLabels,
		ResourceAnnotations: resourceAnnotations,
		PodLabels:           podLabels,
		PodAnnotations:      podAnnotations,
	}
}

// makeDaemonSet creates a DaemonSet with the given configuration (for agents)
func makeDaemonSet(baseName string, namespace string, metadata WorkloadMetadata, podSpec corev1.PodSpec) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        baseName,
			Namespace:   namespace,
			Labels:      metadata.ResourceLabels,
			Annotations: metadata.ResourceAnnotations,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: commonresources.DefaultSelector(baseName),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      metadata.PodLabels,
					Annotations: metadata.PodAnnotations,
				},
				Spec: podSpec,
			},
		},
	}
}

// makeGatewayDaemonSet creates a DaemonSet with UpdateStrategy for gateways
func makeGatewayDaemonSet(baseName string, namespace string, metadata WorkloadMetadata, podSpec corev1.PodSpec) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        baseName,
			Namespace:   namespace,
			Labels:      metadata.ResourceLabels,
			Annotations: metadata.ResourceAnnotations,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: commonresources.DefaultSelector(baseName),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      metadata.PodLabels,
					Annotations: metadata.PodAnnotations,
				},
				Spec: podSpec,
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: new(intstr.FromInt32(0)),
					MaxSurge:       new(intstr.FromInt32(1)),
				},
			},
		},
	}
}

// makeDeployment creates a Deployment with the given configuration
func makeDeployment(baseName string, namespace string, replicas int32, metadata WorkloadMetadata, podSpec corev1.PodSpec) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        baseName,
			Namespace:   namespace,
			Labels:      metadata.ResourceLabels,
			Annotations: metadata.ResourceAnnotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: commonresources.DefaultSelector(baseName),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      metadata.PodLabels,
					Annotations: metadata.PodAnnotations,
				},
				Spec: podSpec,
			},
		},
	}
}
