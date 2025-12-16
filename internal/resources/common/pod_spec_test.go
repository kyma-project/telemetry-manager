package common

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestOverridePodSpecWithTemplate_MergesCorrectly(t *testing.T) {
	original := &corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			ServiceAccountName: "original-sa",
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "nginx:1.21",
				},
			},
		},
	}
	patch := &corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			ServiceAccountName: "patched-sa",
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "foo",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				},
			},
		},
	}

	merged, err := OverridePodSpecWithTemplate(original, patch)
	require.NoError(t, err)
	require.NotNil(t, merged)
	require.Equal(t, "patched-sa", merged.Spec.ServiceAccountName)
	require.Len(t, merged.Spec.Containers, 1)
	require.Equal(t, "foo", merged.Spec.Containers[0].Image)
	require.Equal(t, resource.MustParse("128Mi"), merged.Spec.Containers[0].Resources.Limits[corev1.ResourceMemory])
}
