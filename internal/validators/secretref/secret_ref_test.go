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

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		refs        []telemetryv1alpha1.SecretKeyRef
		expectError error
	}{
		{
			name: "Success",
			refs: []telemetryv1alpha1.SecretKeyRef{
				{Name: "my-secret1", Namespace: "default", Key: "myKey1"},
				{Name: "my-secret2", Namespace: "default", Key: "myKey2"},
			},
			expectError: nil,
		},
		{
			name: "SecretNameNotPresent",
			refs: []telemetryv1alpha1.SecretKeyRef{
				{Name: "my-secret1", Namespace: "default", Key: "myKey1"},
				{Name: "notExistent", Namespace: "default", Key: "myKey2"},
			},
			expectError: ErrSecretRefNotFound,
		},
		{
			name: "SecretNamespaceNotPresent",
			refs: []telemetryv1alpha1.SecretKeyRef{
				{Name: "my-secret1", Namespace: "default", Key: "myKey1"},
				{Name: "my-secret2", Namespace: "notExistent", Key: "myKey2"},
			},
			expectError: ErrSecretRefNotFound,
		},
		{
			name: "SecretKeyNotPresent",
			refs: []telemetryv1alpha1.SecretKeyRef{
				{Name: "my-secret1", Namespace: "default", Key: "myKey1"},
				{Name: "my-secret2", Namespace: "default", Key: "notExistent"},
			},
			expectError: ErrSecretKeyNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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

			client := fake.NewClientBuilder().WithObjects(&existingSecret1).WithObjects(&existingSecret2).Build()

			secretRefValidator := Validator{
				Client: client,
			}
			err := secretRefValidator.Validate(context.TODO(), test.refs)
			require.ErrorIs(t, err, test.expectError)
		})
	}
}

func TestGetValue(t *testing.T) {
	tests := []struct {
		name          string
		refs          telemetryv1alpha1.SecretKeyRef
		expectError   error
		expectedValue string
	}{
		{
			name: "Success",
			refs: telemetryv1alpha1.SecretKeyRef{
				Name: "my-secret1", Namespace: "default", Key: "myKey1",
			},
			expectError:   nil,
			expectedValue: "myValue",
		},
		{
			name: "SecretNameNotPresent",
			refs: telemetryv1alpha1.SecretKeyRef{
				Name: "notExistent", Namespace: "default", Key: "myKey1",
			},
			expectError:   ErrSecretRefNotFound,
			expectedValue: "",
		},
		{
			name: "SecretNamespaceNotPresent",
			refs: telemetryv1alpha1.SecretKeyRef{
				Name: "my-secret1", Namespace: "notExistent", Key: "myKey1",
			},
			expectError:   ErrSecretRefNotFound,
			expectedValue: "",
		},
		{
			name: "SecretKeyNotPresent",
			refs: telemetryv1alpha1.SecretKeyRef{
				Name: "my-secret1", Namespace: "default", Key: "notExistent",
			},
			expectError:   ErrSecretKeyNotFound,
			expectedValue: "",
		},
		{
			name: "SecretRefMissingKey",
			refs: telemetryv1alpha1.SecretKeyRef{
				Name: "my-secret1", Namespace: "default",
			},
			expectError:   ErrSecretRefMissingFields,
			expectedValue: "",
		},
		{
			name: "SecretRefMissingName",
			refs: telemetryv1alpha1.SecretKeyRef{
				Namespace: "default", Key: "notExistent",
			},
			expectError:   ErrSecretRefMissingFields,
			expectedValue: "",
		},
		{
			name: "SecretRefMissingNamespace",
			refs: telemetryv1alpha1.SecretKeyRef{
				Name: "my-secret1", Key: "notExistent",
			},
			expectError:   ErrSecretRefMissingFields,
			expectedValue: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			existingSecret1 := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret1",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"myKey1": []byte("myValue"),
				},
			}

			client := fake.NewClientBuilder().WithObjects(&existingSecret1).Build()

			result, err := GetValue(context.TODO(), client, test.refs)

			require.Equal(t, test.expectedValue, string(result))
			require.ErrorIs(t, err, test.expectError)
		})
	}
}
