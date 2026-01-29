package otelcollector

import (
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/kyma-project/telemetry-manager/internal/config"
)

func TestMakeWorkloadMetadata(t *testing.T) {
	tests := []struct {
		name                   string
		globals                config.Global
		baseName               string
		componentType          string
		extraPodLabels         map[string]string
		annotations            map[string]string
		expectedPodLabels      int
		expectedPodAnnotations int
	}{
		{
			name: "basic metadata without additional labels",
			globals: config.NewGlobal(
				config.WithTargetNamespace("kyma-system"),
			),
			baseName:               "test-collector",
			componentType:          "telemetry",
			expectedPodLabels:      5, // default labels only
			expectedPodAnnotations: 0,
		},
		{
			name: "metadata with additional labels and annotations",
			globals: config.NewGlobal(
				config.WithTargetNamespace("kyma-system"),
				config.WithAdditionalLabels(map[string]string{"custom-label": "value"}),
				config.WithAdditionalAnnotations(map[string]string{"custom-annotation": "annotation-value"}),
			),
			baseName:               "test-collector",
			componentType:          "telemetry",
			expectedPodLabels:      6, // default + 1 custom
			expectedPodAnnotations: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := MakeWorkloadMetadata(&tt.globals, tt.baseName, tt.componentType, tt.extraPodLabels, tt.annotations)

			require.Equal(t, tt.expectedPodLabels, len(metadata.PodLabels))
			require.Equal(t, tt.expectedPodAnnotations, len(metadata.PodAnnotations))
			require.Contains(t, metadata.ResourceLabels, "app.kubernetes.io/name")
			require.Contains(t, metadata.PodLabels, "app.kubernetes.io/name")
		})
	}
}

func TestMakeDaemonSet(t *testing.T) {
	baseName := "test-daemonset"
	namespace := "test-namespace"

	metadata := WorkloadMetadata{
		ResourceLabels:      map[string]string{"resource-label": "value"},
		ResourceAnnotations: map[string]string{"resource-annotation": "value"},
		PodLabels:           map[string]string{"pod-label": "value"},
		PodAnnotations:      map[string]string{"pod-annotation": "value"},
	}

	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "test-container",
				Image: "test-image:latest",
			},
		},
	}

	ds := makeDaemonSet(baseName, namespace, metadata, podSpec)

	require.NotNil(t, ds)
	require.Equal(t, baseName, ds.Name)
	require.Equal(t, namespace, ds.Namespace)
	require.Equal(t, metadata.ResourceLabels, ds.Labels)
	require.Equal(t, metadata.ResourceAnnotations, ds.Annotations)
	require.Equal(t, metadata.PodLabels, ds.Spec.Template.Labels)
	require.Equal(t, metadata.PodAnnotations, ds.Spec.Template.Annotations)
	require.Equal(t, podSpec, ds.Spec.Template.Spec)
	require.NotNil(t, ds.Spec.Selector)
	require.Contains(t, ds.Spec.Selector.MatchLabels, "app.kubernetes.io/name")
}

func TestMakeGatewayDaemonSet(t *testing.T) {
	baseName := "test-gateway-daemonset"
	namespace := "test-namespace"

	metadata := WorkloadMetadata{
		ResourceLabels:      map[string]string{"resource-label": "value"},
		ResourceAnnotations: map[string]string{"resource-annotation": "value"},
		PodLabels:           map[string]string{"pod-label": "value"},
		PodAnnotations:      map[string]string{"pod-annotation": "value"},
	}

	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "gateway-container",
				Image: "gateway-image:latest",
			},
		},
	}

	ds := makeGatewayDaemonSet(baseName, namespace, metadata, podSpec)

	require.NotNil(t, ds)
	require.Equal(t, baseName, ds.Name)
	require.Equal(t, namespace, ds.Namespace)
	require.Equal(t, metadata.ResourceLabels, ds.Labels)
	require.Equal(t, metadata.ResourceAnnotations, ds.Annotations)
	require.Equal(t, metadata.PodLabels, ds.Spec.Template.Labels)
	require.Equal(t, metadata.PodAnnotations, ds.Spec.Template.Annotations)
	require.Equal(t, podSpec, ds.Spec.Template.Spec)

	// Verify UpdateStrategy
	require.Equal(t, appsv1.RollingUpdateDaemonSetStrategyType, ds.Spec.UpdateStrategy.Type)
	require.NotNil(t, ds.Spec.UpdateStrategy.RollingUpdate)
	require.Equal(t, intstr.FromInt32(0), *ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable)
	require.Equal(t, intstr.FromInt32(1), *ds.Spec.UpdateStrategy.RollingUpdate.MaxSurge)
}

func TestMakeDaemonSet_SelectorLabels(t *testing.T) {
	baseName := "test-selector"
	namespace := "test-namespace"

	metadata := WorkloadMetadata{
		PodLabels: map[string]string{
			"app.kubernetes.io/name": baseName,
			"extra-label":            "extra-value",
		},
	}

	podSpec := corev1.PodSpec{}

	ds := makeDaemonSet(baseName, namespace, metadata, podSpec)

	// Selector should only contain the default selector labels, not all pod labels
	require.Contains(t, ds.Spec.Selector.MatchLabels, "app.kubernetes.io/name")
	require.NotContains(t, ds.Spec.Selector.MatchLabels, "extra-label")
}

func TestMakeDeployment_SelectorLabels(t *testing.T) {
	baseName := "test-selector"
	namespace := "test-namespace"

	metadata := WorkloadMetadata{
		PodLabels: map[string]string{
			"app.kubernetes.io/name": baseName,
			"extra-label":            "extra-value",
		},
	}

	podSpec := corev1.PodSpec{}

	deployment := makeDeployment(baseName, namespace, 1, metadata, podSpec)

	// Selector should only contain the default selector labels, not all pod labels
	require.Contains(t, deployment.Spec.Selector.MatchLabels, "app.kubernetes.io/name")
	require.NotContains(t, deployment.Spec.Selector.MatchLabels, "extra-label")
}
