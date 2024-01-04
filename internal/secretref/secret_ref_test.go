package secretref

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type mockGetter struct {
	refs []telemetryv1alpha1.SecretKeyRef
}

func (m mockGetter) GetSecretRefs() []telemetryv1alpha1.SecretKeyRef {
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
		refs: []telemetryv1alpha1.SecretKeyRef{
			{Name: "my-secret1", Namespace: "default", Key: "myKey1"},
			{Name: "my-secret2", Namespace: "default", Key: "myKey2"},
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
		refs: []telemetryv1alpha1.SecretKeyRef{
			{Name: "my-secret1", Namespace: "default", Key: "myKey1"},
			{Name: "my-secret2", Namespace: "default", Key: "myKey2"},
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
		refs: []telemetryv1alpha1.SecretKeyRef{
			{Name: "my-secret1", Namespace: "default", Key: "wrongKey"},
		},
	}

	client := fake.NewClientBuilder().WithObjects(&existingSecret1).Build()

	referencesNonExistentSecret := ReferencesNonExistentSecret(context.TODO(), client, getter)
	require.True(t, referencesNonExistentSecret)
}

func TestReferencesSecret_Success(t *testing.T) {
	getter := mockGetter{
		refs: []telemetryv1alpha1.SecretKeyRef{
			{Name: "my-secret1", Namespace: "default", Key: "myKey"},
		},
	}

	referencesSecret := ReferencesSecret("my-secret1", "default", getter)
	require.True(t, referencesSecret)
}

func TestReferencesSecret_WrongName(t *testing.T) {
	getter := mockGetter{
		refs: []telemetryv1alpha1.SecretKeyRef{
			{Name: "my-secret1", Namespace: "default", Key: "myKey"},
		},
	}

	referencesSecret := ReferencesSecret("wrong-secret-name", "default", getter)
	require.False(t, referencesSecret)
}

func TestReferencesSecret_WrongNamespace(t *testing.T) {
	getter := mockGetter{
		refs: []telemetryv1alpha1.SecretKeyRef{
			{Name: "my-secret1", Namespace: "default", Key: "myKey"},
		},
	}

	referencesSecret := ReferencesSecret("my-secret1", "wrong-namespace", getter)
	require.False(t, referencesSecret)
}

func TestReferencesSecret_NoRefs(t *testing.T) {
	getter := mockGetter{
		refs: []telemetryv1alpha1.SecretKeyRef{},
	}

	referencesSecret := ReferencesSecret("my-secret1", "default", getter)
	require.False(t, referencesSecret)
}

func TestGetValue_Success(t *testing.T) {
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

	result, err := GetValue(context.TODO(), client, telemetryv1alpha1.SecretKeyRef{
		Name:      "my-secret1",
		Namespace: "default",
		Key:       "myKey1",
	})
	require.NoError(t, err)
	require.Equal(t, "myValue1", string(result))
}

func TestGetValue_SecretDoesNotExist(t *testing.T) {
	client := fake.NewClientBuilder().Build()

	result, err := GetValue(context.TODO(), client, telemetryv1alpha1.SecretKeyRef{
		Name:      "my-secret1",
		Namespace: "default",
		Key:       "myKey1",
	})

	require.Error(t, err)
	require.Empty(t, result)
}

func TestGetValue_SecretKeyDoesNotExist(t *testing.T) {
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

	result, err := GetValue(context.TODO(), client, telemetryv1alpha1.SecretKeyRef{
		Name:      "my-secret1",
		Namespace: "default",
		Key:       "wrong-key",
	})
	require.Error(t, err)
	require.Empty(t, result)
}
