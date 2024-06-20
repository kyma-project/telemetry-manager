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

func TestVerifySecretReference_Success(t *testing.T) {
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

	err := VerifySecretReference(context.TODO(), client, getter)
	require.Nil(t, err)
}

func TestVerifySecretReference_SecretNotPresent(t *testing.T) {
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

	err := VerifySecretReference(context.TODO(), client, getter)
	require.ErrorIs(t, err, ErrSecretRefNotFound)
}

func TestVerifySecretReference_KeyNotPresent(t *testing.T) {
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

	err := VerifySecretReference(context.TODO(), client, getter)
	require.ErrorIs(t, err, ErrSecretKeyNotFound)
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

func TestGetValue_SecretDoesNotExistForNamespace(t *testing.T) {
	existingSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret1",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"myKey1": []byte("myValue1"),
		},
	}
	client := fake.NewClientBuilder().WithObjects(&existingSecret).Build()

	result, err := GetValue(context.TODO(), client, telemetryv1alpha1.SecretKeyRef{
		Name:      "my-secret1",
		Namespace: "notExist",
		Key:       "myKey1",
	})

	require.Error(t, err)
	require.Empty(t, result)
}

func TestGetValue_SecretDoesNotExistForName(t *testing.T) {
	existingSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret1",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"myKey1": []byte("myValue1"),
		},
	}
	client := fake.NewClientBuilder().WithObjects(&existingSecret).Build()

	result, err := GetValue(context.TODO(), client, telemetryv1alpha1.SecretKeyRef{
		Name:      "notExist",
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
