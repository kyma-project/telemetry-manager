package secretref

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/telemetry-manager/internal/field"
)

type mockGetter struct {
	refs []field.Descriptor
}

func (m mockGetter) GetSecretRefs() []field.Descriptor {
	return m.refs
}

func TestReferencesNonExistentSecret_Success(t *testing.T) {
	existingSecret1 := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret1",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"myKey1": []byte("myValue"),
		},
	}
	existingSecret2 := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret2",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"myKey2": []byte("myValue"),
		},
	}

	getter := mockGetter{
		refs: []field.Descriptor{
			{
				SourceSecretName:      "my-secret1",
				SourceSecretNamespace: "default",
				SourceSecretKey:       "myKey1",
			},
			{
				SourceSecretName:      "my-secret2",
				SourceSecretNamespace: "default",
				SourceSecretKey:       "myKey2",
			},
		},
	}

	client := fake.NewClientBuilder().WithObjects(&existingSecret1).WithObjects(&existingSecret2).Build()

	referencesNonExistentSecret := ReferencesNonExistentSecret(context.TODO(), client, getter)
	require.False(t, referencesNonExistentSecret)
}

func TestReferencesNonExistentSecret_SecretNotPresent(t *testing.T) {
	existingSecret1 := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret1",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"myKey1": []byte("myValue"),
		},
	}

	getter := mockGetter{
		refs: []field.Descriptor{
			{
				SourceSecretName:      "my-secret1",
				SourceSecretNamespace: "default",
				SourceSecretKey:       "myKey1",
			},
			{
				SourceSecretName:      "my-secret2",
				SourceSecretNamespace: "default",
				SourceSecretKey:       "myKey2",
			},
		},
	}

	client := fake.NewClientBuilder().WithObjects(&existingSecret1).Build()

	referencesNonExistentSecret := ReferencesNonExistentSecret(context.TODO(), client, getter)
	require.True(t, referencesNonExistentSecret)
}

func TestReferencesNonExistentSecret_KeyNotPresent(t *testing.T) {
	existingSecret1 := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret1",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"myKey1": []byte("myValue"),
		},
	}

	getter := mockGetter{
		refs: []field.Descriptor{
			{
				SourceSecretName:      "my-secret1",
				SourceSecretNamespace: "default",
				SourceSecretKey:       "wrongKey",
			},
		},
	}

	client := fake.NewClientBuilder().WithObjects(&existingSecret1).Build()

	referencesNonExistentSecret := ReferencesNonExistentSecret(context.TODO(), client, getter)
	require.True(t, referencesNonExistentSecret)
}

func TestReferencesSecret_Success(t *testing.T) {
	getter := mockGetter{
		refs: []field.Descriptor{
			{
				SourceSecretName:      "my-secret1",
				SourceSecretNamespace: "default",
				SourceSecretKey:       "myKey",
			},
		},
	}

	referencesSecret := ReferencesSecret("my-secret1", "default", getter)
	require.True(t, referencesSecret)
}

func TestReferencesSecret_WrongName(t *testing.T) {
	getter := mockGetter{
		refs: []field.Descriptor{
			{
				SourceSecretName:      "my-secret1",
				SourceSecretNamespace: "default",
				SourceSecretKey:       "myKey",
			},
		},
	}

	referencesSecret := ReferencesSecret("wrong-secret-name", "default", getter)
	require.False(t, referencesSecret)
}

func TestReferencesSecret_WrongNamespace(t *testing.T) {
	getter := mockGetter{
		refs: []field.Descriptor{
			{

				SourceSecretName:      "my-secret1",
				SourceSecretNamespace: "default",
				SourceSecretKey:       "myKey",
			},
		},
	}

	referencesSecret := ReferencesSecret("my-secret1", "wrong-namespace", getter)
	require.False(t, referencesSecret)
}

func TestReferencesSecret_NoRefs(t *testing.T) {
	getter := mockGetter{
		refs: []field.Descriptor{},
	}

	referencesSecret := ReferencesSecret("my-secret1", "default", getter)
	require.False(t, referencesSecret)
}

func TestFetchReferencedSecretData_Success(t *testing.T) {
	existingSecret1 := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret1",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"myKey1": []byte("myValue1"),
		},
	}
	existingSecret2 := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret2",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"myKey2": []byte("myValue2"),
		},
	}

	client := fake.NewClientBuilder().WithObjects(&existingSecret1).WithObjects(&existingSecret2).Build()

	getter := mockGetter{
		refs: []field.Descriptor{
			{
				TargetSecretKey:       "myKey1",
				SourceSecretName:      "my-secret1",
				SourceSecretNamespace: "default",
				SourceSecretKey:       "myKey1",
			},
			{
				TargetSecretKey:       "myKey2",
				SourceSecretName:      "my-secret2",
				SourceSecretNamespace: "default",
				SourceSecretKey:       "myKey2",
			},
		},
	}

	fetchedData, err := FetchReferencedSecretData(context.TODO(), client, getter)

	require.Nil(t, err)
	require.Equal(t, 2, len(fetchedData))
	require.Equal(t, "myValue1", string(fetchedData["myKey1"]))
	require.Equal(t, "myValue2", string(fetchedData["myKey2"]))
}

func TestFetchReferencedSecretData_SecretDoesNotExist(t *testing.T) {
	client := fake.NewClientBuilder().Build()

	getter := mockGetter{
		refs: []field.Descriptor{
			{
				TargetSecretKey:       "my-secret1",
				SourceSecretName:      "my-secret1",
				SourceSecretNamespace: "default",
				SourceSecretKey:       "myKey1",
			},
		},
	}

	fetchedData, err := FetchReferencedSecretData(context.TODO(), client, getter)

	require.Error(t, err)
	require.Nil(t, fetchedData)
}

func TestFetchReferencedSecretData_SecretKeyDoesNotExist(t *testing.T) {
	existingSecret1 := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret1",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"myKey1": []byte("myValue1"),
		},
	}
	client := fake.NewClientBuilder().WithObjects(&existingSecret1).Build()

	getter := mockGetter{
		refs: []field.Descriptor{
			{
				TargetSecretKey:       "my-secret1",
				SourceSecretName:      "my-secret1",
				SourceSecretNamespace: "default",
				SourceSecretKey:       "wrong-key",
			},
		},
	}

	fetchedData, err := FetchReferencedSecretData(context.TODO(), client, getter)
	require.Error(t, err)
	require.Nil(t, fetchedData)
}
