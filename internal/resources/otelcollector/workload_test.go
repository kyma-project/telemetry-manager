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
		extraPodAnnotations    map[string]string
		expectedPodLabels      int
		expectedPodAnnotations int
		expectedResourceLabels int
		expectedResourceAnnotations int
		verifyPodLabels        map[string]string
		verifyPodAnnotations   map[string]string
	}{
		{
			name: "basic metadata without additional labels",
			globals: config.NewGlobal(
				config.WithTargetNamespace("kyma-system"),
			),
			baseName:                    "test-collector",
			componentType:               "telemetry",
			expectedPodLabels:           5, // default labels only
			expectedPodAnnotations:      0,
			expectedResourceLabels:      0,
			expectedResourceAnnotations: 0,
		},
		{
			name: "metadata with workload labels and annotations only",
			globals: config.NewGlobal(
				config.WithTargetNamespace("kyma-system"),
				config.WithAdditionalWorkloadLabels(map[string]string{"workload-label": "workload-value"}),
				config.WithAdditionalWorkloadAnnotations(map[string]string{"workload-annotation": "workload-annotation-value"}),
			),
			baseName:                    "test-collector",
			componentType:               "telemetry",
			expectedPodLabels:           6, // default + 1 workload label
			expectedPodAnnotations:      1, // 1 workload annotation
			expectedResourceLabels:      1, // 1 workload label
			expectedResourceAnnotations: 1, // 1 workload annotation
			verifyPodLabels: map[string]string{
				"workload-label": "workload-value",
			},
			verifyPodAnnotations: map[string]string{
				"workload-annotation": "workload-annotation-value",
			},
		},
		{
			name: "metadata with pod-specific labels and annotations",
			globals: config.NewGlobal(
				config.WithTargetNamespace("kyma-system"),
				config.WithAdditionalWorkloadPodLabels(map[string]string{"pod-label": "pod-value"}),
				config.WithAdditionalWorkloadPodAnnotations(map[string]string{"pod-annotation": "pod-annotation-value"}),
			),
			baseName:                    "test-collector",
			componentType:               "telemetry",
			expectedPodLabels:           6, // default + 1 pod label
			expectedPodAnnotations:      1, // 1 pod annotation
			expectedResourceLabels:      0, // no workload labels
			expectedResourceAnnotations: 0, // no workload annotations
			verifyPodLabels: map[string]string{
				"pod-label": "pod-value",
			},
			verifyPodAnnotations: map[string]string{
				"pod-annotation": "pod-annotation-value",
			},
		},
		{
			name: "metadata with both workload and pod-specific labels/annotations",
			globals: config.NewGlobal(
				config.WithTargetNamespace("kyma-system"),
				config.WithAdditionalWorkloadLabels(map[string]string{"workload-label": "workload-value"}),
				config.WithAdditionalWorkloadAnnotations(map[string]string{"workload-annotation": "workload-annotation-value"}),
				config.WithAdditionalWorkloadPodLabels(map[string]string{"pod-label": "pod-value"}),
				config.WithAdditionalWorkloadPodAnnotations(map[string]string{"pod-annotation": "pod-annotation-value"}),
			),
			baseName:                    "test-collector",
			componentType:               "telemetry",
			expectedPodLabels:           7, // default + 1 workload label + 1 pod label
			expectedPodAnnotations:      2, // 1 workload annotation + 1 pod annotation
			expectedResourceLabels:      1, // 1 workload label
			expectedResourceAnnotations: 1, // 1 workload annotation
			verifyPodLabels: map[string]string{
				"workload-label": "workload-value",
				"pod-label":      "pod-value",
			},
			verifyPodAnnotations: map[string]string{
				"workload-annotation": "workload-annotation-value",
				"pod-annotation":      "pod-annotation-value",
			},
		},
		{
			name: "metadata with extra pod labels and annotations from workload",
			globals: config.NewGlobal(
				config.WithTargetNamespace("kyma-system"),
				config.WithAdditionalWorkloadLabels(map[string]string{"workload-label": "value"}),
			),
			baseName:      "test-collector",
			componentType: "telemetry",
			extraPodLabels: map[string]string{
				"istio-inject": "true",
			},
			extraPodAnnotations: map[string]string{
				"sidecar.istio.io/inject": "true",
			},
			expectedPodLabels:           7, // default + 1 workload + 1 extra
			expectedPodAnnotations:      1, // 1 extra
			expectedResourceLabels:      1,
			expectedResourceAnnotations: 0,
			verifyPodLabels: map[string]string{
				"workload-label": "value",
				"istio-inject":   "true",
			},
			verifyPodAnnotations: map[string]string{
				"sidecar.istio.io/inject": "true",
			},
		},
		{
			name: "metadata layering order - extra labels override pod labels",
			globals: config.NewGlobal(
				config.WithTargetNamespace("kyma-system"),
				config.WithAdditionalWorkloadLabels(map[string]string{"env": "workload"}),
				config.WithAdditionalWorkloadPodLabels(map[string]string{"env": "pod"}),
			),
			baseName:      "test-collector",
			componentType: "telemetry",
			extraPodLabels: map[string]string{
				"env": "extra", // should override pod and workload labels
			},
			expectedPodLabels:      6, // default + 1 (env key used 3 times but only counts once)
			expectedResourceLabels: 1,
			verifyPodLabels: map[string]string{
				"env": "extra", // extra takes precedence
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := MakeWorkloadMetadata(&tt.globals, tt.baseName, tt.componentType, tt.extraPodLabels, tt.extraPodAnnotations)

			require.Equal(t, tt.expectedPodLabels, len(metadata.PodLabels), "pod labels count mismatch")
			require.Equal(t, tt.expectedPodAnnotations, len(metadata.PodAnnotations), "pod annotations count mismatch")
			require.Equal(t, tt.expectedResourceLabels, len(metadata.ResourceLabels), "resource labels count mismatch")
			require.Equal(t, tt.expectedResourceAnnotations, len(metadata.ResourceAnnotations), "resource annotations count mismatch")

			// Verify default labels are always present
			require.Contains(t, metadata.PodLabels, "app.kubernetes.io/name")

			// Verify specific labels/annotations if provided
			for k, v := range tt.verifyPodLabels {
				require.Equal(t, v, metadata.PodLabels[k], "pod label %s should equal %s", k, v)
			}
			for k, v := range tt.verifyPodAnnotations {
				require.Equal(t, v, metadata.PodAnnotations[k], "pod annotation %s should equal %s", k, v)
			}
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
