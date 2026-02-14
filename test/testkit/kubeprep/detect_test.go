package kubeprep

import (
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDetectIstioInstalled(t *testing.T) {
	tests := []struct {
		name     string
		objects  []client.Object
		expected bool
	}{
		{
			name: "istio installed - default CR exists",
			objects: []client.Object{
				createIstioCR("default", "kyma-system"),
			},
			expected: true,
		},
		{
			name:     "istio not installed - no CR",
			objects:  []client.Object{},
			expected: false,
		},
		{
			name: "istio CR in wrong namespace - not detected",
			objects: []client.Object{
				createIstioCR("default", "wrong-namespace"),
			},
			expected: false,
		},
		{
			name: "istio CR with wrong name - not detected",
			objects: []client.Object{
				createIstioCR("wrong-name", "kyma-system"),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()

			actual := detectIstioInstalled(t, client)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestDetectFIPSMode(t *testing.T) {
	tests := []struct {
		name     string
		objects  []client.Object
		expected bool
	}{
		{
			name: "fips mode enabled",
			objects: []client.Object{
				createManagerDeployment(map[string]string{
					"OPERATE_IN_FIPS_MODE": "true",
				}),
			},
			expected: true,
		},
		{
			name: "fips mode disabled",
			objects: []client.Object{
				createManagerDeployment(map[string]string{
					"OPERATE_IN_FIPS_MODE": "false",
				}),
			},
			expected: false,
		},
		{
			name: "fips env var not set",
			objects: []client.Object{
				createManagerDeployment(map[string]string{}),
			},
			expected: false,
		},
		{
			name:     "manager not deployed",
			objects:  []client.Object{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = appsv1.AddToScheme(scheme)

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()

			actual := detectFIPSMode(t, client)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestDetectExperimentalEnabled(t *testing.T) {
	tests := []struct {
		name             string
		objects          []client.Object
		expectedEnabled  bool
		expectedHasLabel bool
	}{
		{
			name: "experimental enabled - label true",
			objects: []client.Object{
				createManagerDeploymentWithLabels(nil, map[string]string{
					LabelExperimentalEnabled: "true",
				}),
			},
			expectedEnabled:  true,
			expectedHasLabel: true,
		},
		{
			name: "experimental disabled - label false",
			objects: []client.Object{
				createManagerDeploymentWithLabels(nil, map[string]string{
					LabelExperimentalEnabled: "false",
				}),
			},
			expectedEnabled:  false,
			expectedHasLabel: true,
		},
		{
			name: "label missing - state unknown",
			objects: []client.Object{
				createManagerDeploymentWithLabels(nil, map[string]string{}),
			},
			expectedEnabled:  false,
			expectedHasLabel: false,
		},
		{
			name:             "manager not deployed",
			objects:          []client.Object{},
			expectedEnabled:  false,
			expectedHasLabel: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = appsv1.AddToScheme(scheme)

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()

			actualEnabled, actualHasLabel := detectExperimentalEnabled(t, client)
			require.Equal(t, tt.expectedEnabled, actualEnabled, "experimental enabled mismatch")
			require.Equal(t, tt.expectedHasLabel, actualHasLabel, "has label mismatch")
		})
	}
}

func TestDetectManagerDeployed(t *testing.T) {
	tests := []struct {
		name     string
		objects  []client.Object
		expected bool
	}{
		{
			name: "manager deployed",
			objects: []client.Object{
				createManagerDeployment(map[string]string{}),
			},
			expected: true,
		},
		{
			name:     "manager not deployed",
			objects:  []client.Object{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = appsv1.AddToScheme(scheme)

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()

			actual := detectManagerDeployed(t, client)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestDetectClusterState(t *testing.T) {
	tests := []struct {
		name     string
		objects  []client.Object
		expected Config
	}{
		{
			name: "fresh cluster - nothing installed",
			objects: []client.Object{
				// Empty cluster
			},
			expected: Config{
				ManagerImage:            "",
				LocalImage:              false,
				InstallIstio:            false,
				OperateInFIPSMode:       false,
				EnableExperimental:      false,
				CustomLabelsAnnotations: false,
				SkipManagerDeployment:   true, // Manager not deployed
				SkipPrerequisites:       false,
				NeedsReinstall:          false, // No manager, no reinstall needed
			},
		},
		{
			name: "manager deployed with label - no reinstall needed",
			objects: []client.Object{
				createManagerDeploymentWithLabels(
					map[string]string{"OPERATE_IN_FIPS_MODE": "false"},
					map[string]string{LabelExperimentalEnabled: "false"},
				),
			},
			expected: Config{
				ManagerImage:            "",
				LocalImage:              false,
				InstallIstio:            false,
				OperateInFIPSMode:       false,
				EnableExperimental:      false,
				CustomLabelsAnnotations: false,
				SkipManagerDeployment:   false, // Manager is deployed
				SkipPrerequisites:       false,
				NeedsReinstall:          false, // Label exists
			},
		},
		{
			name: "manager deployed without label - needs reinstall",
			objects: []client.Object{
				createManagerDeployment(map[string]string{
					"OPERATE_IN_FIPS_MODE": "false",
				}),
			},
			expected: Config{
				ManagerImage:            "",
				LocalImage:              false,
				InstallIstio:            false,
				OperateInFIPSMode:       false,
				EnableExperimental:      false,
				CustomLabelsAnnotations: false,
				SkipManagerDeployment:   false, // Manager is deployed
				SkipPrerequisites:       false,
				NeedsReinstall:          true, // Label missing
			},
		},
		{
			name: "istio installed",
			objects: []client.Object{
				createIstioCR("default", "kyma-system"),
				createManagerDeploymentWithLabels(
					map[string]string{"OPERATE_IN_FIPS_MODE": "false"},
					map[string]string{LabelExperimentalEnabled: "false"},
				),
			},
			expected: Config{
				ManagerImage:            "",
				LocalImage:              false,
				InstallIstio:            true,
				OperateInFIPSMode:       false,
				EnableExperimental:      false,
				CustomLabelsAnnotations: false,
				SkipManagerDeployment:   false,
				SkipPrerequisites:       false,
				NeedsReinstall:          false,
			},
		},
		{
			name: "fips mode enabled",
			objects: []client.Object{
				createManagerDeploymentWithLabels(
					map[string]string{"OPERATE_IN_FIPS_MODE": "true"},
					map[string]string{LabelExperimentalEnabled: "false"},
				),
			},
			expected: Config{
				ManagerImage:            "",
				LocalImage:              false,
				InstallIstio:            false,
				OperateInFIPSMode:       true,
				EnableExperimental:      false,
				CustomLabelsAnnotations: false,
				SkipManagerDeployment:   false,
				SkipPrerequisites:       false,
				NeedsReinstall:          false,
			},
		},
		{
			name: "experimental features enabled",
			objects: []client.Object{
				createManagerDeploymentWithLabels(
					map[string]string{"OPERATE_IN_FIPS_MODE": "false"},
					map[string]string{LabelExperimentalEnabled: "true"},
				),
			},
			expected: Config{
				ManagerImage:            "",
				LocalImage:              false,
				InstallIstio:            false,
				OperateInFIPSMode:       false,
				EnableExperimental:      true,
				CustomLabelsAnnotations: false,
				SkipManagerDeployment:   false,
				SkipPrerequisites:       false,
				NeedsReinstall:          false,
			},
		},
		{
			name: "full setup - istio, fips, experimental",
			objects: []client.Object{
				createIstioCR("default", "kyma-system"),
				createManagerDeploymentWithLabels(
					map[string]string{"OPERATE_IN_FIPS_MODE": "true"},
					map[string]string{LabelExperimentalEnabled: "true"},
				),
			},
			expected: Config{
				ManagerImage:            "",
				LocalImage:              false,
				InstallIstio:            true,
				OperateInFIPSMode:       true,
				EnableExperimental:      true,
				CustomLabelsAnnotations: false,
				SkipManagerDeployment:   false,
				SkipPrerequisites:       false,
				NeedsReinstall:          false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = appsv1.AddToScheme(scheme)

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()

			actual, err := DetectClusterState(t, client)
			require.NoError(t, err)
			require.Equal(t, tt.expected, *actual)
		})
	}
}

func TestDetectOrUseProvidedConfig(t *testing.T) {
	t.Run("uses provided config when available", func(t *testing.T) {
		providedConfig := &Config{
			ManagerImage:       "custom-image:v1",
			InstallIstio:       true,
			OperateInFIPSMode:  true,
			EnableExperimental: true,
		}

		scheme := runtime.NewScheme()
		client := fake.NewClientBuilder().WithScheme(scheme).Build()

		result, err := DetectOrUseProvidedConfig(t, client, providedConfig)
		require.NoError(t, err)
		require.Equal(t, providedConfig, result)
	})

	t.Run("detects cluster state when no config provided - with label", func(t *testing.T) {
		scheme := runtime.NewScheme()
		_ = appsv1.AddToScheme(scheme)

		objects := []client.Object{
			createManagerDeploymentWithLabels(
				map[string]string{"OPERATE_IN_FIPS_MODE": "true"},
				map[string]string{LabelExperimentalEnabled: "false"},
			),
		}

		client := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(objects...).
			Build()

		result, err := DetectOrUseProvidedConfig(t, client, nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.True(t, result.OperateInFIPSMode)
		require.False(t, result.SkipManagerDeployment)
		require.False(t, result.NeedsReinstall) // Label exists
	})

	t.Run("detects cluster state when no config provided - without label", func(t *testing.T) {
		scheme := runtime.NewScheme()
		_ = appsv1.AddToScheme(scheme)

		objects := []client.Object{
			createManagerDeployment(map[string]string{
				"OPERATE_IN_FIPS_MODE": "true",
			}),
		}

		client := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(objects...).
			Build()

		result, err := DetectOrUseProvidedConfig(t, client, nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.True(t, result.OperateInFIPSMode)
		require.False(t, result.SkipManagerDeployment)
		require.True(t, result.NeedsReinstall) // Label missing
	})
}

// Helper functions to create test objects

func createIstioCR(name, namespace string) *unstructured.Unstructured {
	cr := &unstructured.Unstructured{}
	cr.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.kyma-project.io",
		Version: "v1alpha2",
		Kind:    "Istio",
	})
	cr.SetName(name)
	cr.SetNamespace(namespace)
	return cr
}

func createManagerDeployment(envVars map[string]string) *appsv1.Deployment {
	return createManagerDeploymentWithLabels(envVars, nil)
}

func createManagerDeploymentWithLabels(envVars map[string]string, labels map[string]string) *appsv1.Deployment {
	env := []corev1.EnvVar{}
	for k, v := range envVars {
		env = append(env, corev1.EnvVar{Name: k, Value: v})
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      telemetryManagerName,
			Namespace: kymaSystemNamespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "manager",
							Env:  env,
						},
					},
				},
			},
		},
	}
}

func createCRD(name string) *unstructured.Unstructured {
	crd := &unstructured.Unstructured{}
	crd.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apiextensions.k8s.io",
		Version: "v1",
		Kind:    "CustomResourceDefinition",
	})
	crd.SetName(name)
	return crd
}

// TestDetectClusterState_ErrorHandling tests error cases
func TestDetectClusterState_ErrorHandling(t *testing.T) {
	t.Run("handles client errors gracefully", func(t *testing.T) {
		// Create a client that will return errors
		scheme := runtime.NewScheme()
		client := fake.NewClientBuilder().
			WithScheme(scheme).
			Build()

		// Even with errors, should return a config with defaults
		config, err := DetectClusterState(t, client)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Should default to safe values
		require.False(t, config.InstallIstio)
		require.False(t, config.OperateInFIPSMode)
		require.False(t, config.EnableExperimental)
	})
}
