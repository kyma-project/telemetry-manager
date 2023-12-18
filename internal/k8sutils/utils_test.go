package k8sutils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/telemetry-manager/internal/k8sutils/mocks"
)

func TestGetOrCreateConfigMapError(t *testing.T) {
	mockClient := &mocks.Client{}
	badReqErr := apierrors.NewBadRequest("")
	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(badReqErr)

	configMapName := types.NamespacedName{Name: "some-cm", Namespace: "cm-ns"}
	_, err := GetOrCreateConfigMap(context.Background(), mockClient, configMapName)

	require.Error(t, err)
	require.Equal(t, badReqErr, err)
}

func TestGetOrCreateConfigMapGetSuccess(t *testing.T) {
	mockClient := &mocks.Client{}
	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	configMapName := types.NamespacedName{Name: "some-cm", Namespace: "cm-ns"}
	cm, err := GetOrCreateConfigMap(context.Background(), mockClient, configMapName)

	require.NoError(t, err)
	require.Equal(t, "some-cm", cm.Name)
	require.Equal(t, "cm-ns", cm.Namespace)
}

func TestGetOrCreateConfigMapCreateSuccess(t *testing.T) {
	mockClient := &mocks.Client{}
	notFoundErr := apierrors.NewNotFound(schema.GroupResource{}, "")
	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(notFoundErr)
	mockClient.On("Create", mock.Anything, mock.Anything).Return(nil)

	configMapName := types.NamespacedName{Name: "some-cm", Namespace: "cm-ns"}
	cm, err := GetOrCreateConfigMap(context.Background(), mockClient, configMapName)

	require.NoError(t, err)
	require.Equal(t, "some-cm", cm.Name)
	require.Equal(t, "cm-ns", cm.Namespace)
}

func TestGetOrCreateSecretError(t *testing.T) {
	mockClient := &mocks.Client{}
	badReqErr := apierrors.NewBadRequest("")
	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(badReqErr)

	secretName := types.NamespacedName{Name: "some-secret", Namespace: "secret-ns"}
	_, err := GetOrCreateSecret(context.Background(), mockClient, secretName)

	require.Error(t, err)
	require.Equal(t, badReqErr, err)
}

func TestGetOrCreateSecretSuccess(t *testing.T) {
	mockClient := &mocks.Client{}
	notFoundErr := apierrors.NewNotFound(schema.GroupResource{}, "")
	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(notFoundErr)
	mockClient.On("Create", mock.Anything, mock.Anything).Return(nil)

	secretName := types.NamespacedName{Name: "some-secret", Namespace: "secret-ns"}
	secret, err := GetOrCreateSecret(context.Background(), mockClient, secretName)

	require.NoError(t, err)
	require.Equal(t, "some-secret", secret.Name)
	require.Equal(t, "secret-ns", secret.Namespace)
}

func TestMergePodAnnotations(t *testing.T) {
	tests := []struct {
		name           string
		existing       map[string]string
		desired        map[string]string
		expectedMerged map[string]string
	}{
		{
			name: "should preserve existing kubectl annotations",
			existing: map[string]string{
				"kubectl.kubernetes.io/1": "1",
				"kubectl.kubernetes.io/2": "2",
				"kubectl.kubernetes.io/3": "3",
				"unrelated":               "foo",
			},
			desired: map[string]string{
				"kubectl.kubernetes.io/2": "b",
				"kubectl.kubernetes.io/3": "3",
				"kubectl.kubernetes.io/4": "4",
			},
			expectedMerged: map[string]string{
				"kubectl.kubernetes.io/1": "1",
				"kubectl.kubernetes.io/2": "b",
				"kubectl.kubernetes.io/3": "3",
				"kubectl.kubernetes.io/4": "4",
			},
		},
		{
			name: "should preserve existing checksum annotations",
			existing: map[string]string{
				"checksum/1": "1",
				"checksum/2": "2",
				"checksum/3": "3",
				"unrelated":  "foo",
			},
			desired: map[string]string{
				"checksum/2": "b",
				"checksum/3": "3",
				"checksum/4": "4",
			},
			expectedMerged: map[string]string{
				"checksum/1": "1",
				"checksum/2": "b",
				"checksum/3": "3",
				"checksum/4": "4",
			},
		},
		{
			name: "should preserve existing istio restartedAt annotations",
			existing: map[string]string{
				"istio-operator.kyma-project.io/restartedAt": "2023-08-17T13:36:09Z",
				"unrelated": "foo",
			},
			desired: map[string]string{
				"checksum/1": "1",
			},
			expectedMerged: map[string]string{
				"istio-operator.kyma-project.io/restartedAt": "2023-08-17T13:36:09Z",
				"checksum/1": "1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existing := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "some-deployment",
					Annotations: tt.existing,
				},
			}

			desired := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "some-deployment",
					Annotations: tt.desired,
				},
			}

			mergePodAnnotations(&desired.ObjectMeta, existing.ObjectMeta)

			require.Equal(t, len(tt.expectedMerged), len(desired.Annotations))

			for k, v := range tt.expectedMerged {
				require.Contains(t, desired.Annotations, k)
				require.Equal(t, v, desired.Annotations[k])
			}
		})
	}
}

func TestMergeOwnerReference(t *testing.T) {
	oldOwners := []metav1.OwnerReference{
		{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "old-deployment-1",
			UID:        "old-deployment-uid-1",
		},
		{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "old-deployment-2",
			UID:        "old-deployment-uid-2",
		},
	}
	newOwners := []metav1.OwnerReference{
		{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "new-deployment-1",
			UID:        "new-deployment-uid-1",
		},
	}

	merged := mergeOwnerReferences(newOwners, oldOwners)
	require.Equal(t, 3, len(merged))
}
