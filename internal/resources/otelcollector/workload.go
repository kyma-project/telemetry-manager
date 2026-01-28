package otelcollector

import (
	"maps"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

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

// MakeWorkloadMetadata creates metadata for workloads (Deployment or DaemonSet)
func MakeWorkloadMetadata(globals *config.Global, baseName string, componentType string, extraPodLabels map[string]string, annotations map[string]string) WorkloadMetadata {
	defaultLabels := commonresources.MakeDefaultLabels(baseName, componentType)

	// Create final labels with additional labels from globals
	resourceLabels := make(map[string]string)
	podLabels := make(map[string]string)

	maps.Copy(resourceLabels, globals.AdditionalLabels())
	maps.Copy(podLabels, globals.AdditionalLabels())
	maps.Copy(resourceLabels, defaultLabels)
	maps.Copy(podLabels, defaultLabels)
	maps.Copy(podLabels, extraPodLabels)

	// Create final annotations with additional annotations from globals
	resourceAnnotations := make(map[string]string)
	podAnnotations := make(map[string]string)

	maps.Copy(resourceAnnotations, globals.AdditionalAnnotations())
	maps.Copy(podAnnotations, globals.AdditionalAnnotations())
	maps.Copy(podAnnotations, annotations)

	return WorkloadMetadata{
		ResourceLabels:      resourceLabels,
		ResourceAnnotations: resourceAnnotations,
		PodLabels:           podLabels,
		PodAnnotations:      podAnnotations,
	}
}

// MakeDaemonSet creates a DaemonSet with the given configuration (for agents)
func MakeDaemonSet(baseName string, namespace string, metadata WorkloadMetadata, podSpec corev1.PodSpec) *appsv1.DaemonSet {
	selectorLabels := commonresources.MakeDefaultSelectorLabels(baseName)

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        baseName,
			Namespace:   namespace,
			Labels:      metadata.ResourceLabels,
			Annotations: metadata.ResourceAnnotations,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
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

// MakeGatewayDaemonSet creates a DaemonSet with UpdateStrategy for gateways
func MakeGatewayDaemonSet(baseName string, namespace string, metadata WorkloadMetadata, podSpec corev1.PodSpec) *appsv1.DaemonSet {
	selectorLabels := commonresources.MakeDefaultSelectorLabels(baseName)

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        baseName,
			Namespace:   namespace,
			Labels:      metadata.ResourceLabels,
			Annotations: metadata.ResourceAnnotations,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
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
					MaxUnavailable: ptr.To[intstr.IntOrString](intstr.FromInt32(0)),
					MaxSurge:       ptr.To[intstr.IntOrString](intstr.FromInt32(1)),
				},
			},
		},
	}
}

// MakeDeployment creates a Deployment with the given configuration
func MakeDeployment(baseName string, namespace string, replicas int32, metadata WorkloadMetadata, podSpec corev1.PodSpec) *appsv1.Deployment {
	selectorLabels := commonresources.MakeDefaultSelectorLabels(baseName)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        baseName,
			Namespace:   namespace,
			Labels:      metadata.ResourceLabels,
			Annotations: metadata.ResourceAnnotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
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
