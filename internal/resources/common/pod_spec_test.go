package common

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestOverridePodSpecWithTemplate_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		original *corev1.PodTemplateSpec
		patch    *corev1.PodTemplateSpec
		verify   func(*testing.T, *corev1.PodTemplateSpec)
	}{
		{
			name: "merges service account and container image",
			original: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "original-sa",
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "nginx:1.21",
						},
					},
				},
			},
			patch: &corev1.PodTemplateSpec{
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
			},
			verify: func(t *testing.T, merged *corev1.PodTemplateSpec) {
				require.Equal(t, "patched-sa", merged.Spec.ServiceAccountName)
				require.Len(t, merged.Spec.Containers, 1)
				require.Equal(t, "foo", merged.Spec.Containers[0].Image)
				require.Equal(t, resource.MustParse("128Mi"), merged.Spec.Containers[0].Resources.Limits[corev1.ResourceMemory])
			},
		},
		{
			name: "merges image pull secrets",
			original: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "app", Image: "nginx"},
					},
				},
			},
			patch: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "image-pull-secret"},
					},
				},
			},
			verify: func(t *testing.T, merged *corev1.PodTemplateSpec) {
				require.Len(t, merged.Spec.ImagePullSecrets, 1)
				require.Equal(t, "image-pull-secret", merged.Spec.ImagePullSecrets[0].Name)
			},
		},
		{
			name: "merges volume mounts",
			original: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "nginx",
						},
					},
				},
			},
			patch: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "app",
							VolumeMounts: []corev1.VolumeMount{
								{Name: "ca-bundle", MountPath: "/ca-bundle"},
							},
						},
					},
				},
			},
			verify: func(t *testing.T, merged *corev1.PodTemplateSpec) {
				require.Len(t, merged.Spec.Containers, 1)
				require.Len(t, merged.Spec.Containers[0].VolumeMounts, 1)
				require.Equal(t, "ca-bundle", merged.Spec.Containers[0].VolumeMounts[0].Name)
				require.Equal(t, "/ca-bundle", merged.Spec.Containers[0].VolumeMounts[0].MountPath)
			},
		},
		{
			name: "appends multiple image pull secrets",
			original: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "secret1"},
					},
					Containers: []corev1.Container{
						{Name: "app", Image: "nginx"},
					},
				},
			},
			patch: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "secret1"},
						{Name: "secret2"},
					},
				},
			},
			verify: func(t *testing.T, merged *corev1.PodTemplateSpec) {
				require.Len(t, merged.Spec.ImagePullSecrets, 2)
				require.Equal(t, "secret1", merged.Spec.ImagePullSecrets[0].Name)
				require.Equal(t, "secret2", merged.Spec.ImagePullSecrets[1].Name)
			},
		},
		{
			name: "merges volumes and volume mounts together",
			original: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{Name: "vol1"},
					},
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "nginx",
						},
					},
				},
			},
			patch: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{Name: "vol1"},
						{Name: "vol2"},
					},
					Containers: []corev1.Container{
						{
							Name: "app",
							VolumeMounts: []corev1.VolumeMount{
								{Name: "vol2", MountPath: "/config"},
							},
						},
					},
				},
			},
			verify: func(t *testing.T, merged *corev1.PodTemplateSpec) {
				require.Len(t, merged.Spec.Volumes, 2)
				require.Equal(t, "vol1", merged.Spec.Volumes[0].Name)
				require.Equal(t, "vol2", merged.Spec.Volumes[1].Name)
				require.Len(t, merged.Spec.Containers[0].VolumeMounts, 1)
				require.Equal(t, "vol2", merged.Spec.Containers[0].VolumeMounts[0].Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged, err := OverridePodSpecWithTemplate(tt.patch, tt.original)
			require.NoError(t, err)
			require.NotNil(t, merged)
			tt.verify(t, merged)
		})
	}
}
