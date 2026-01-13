package secretref

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		refs        []telemetryv1beta1.SecretKeyRef
		expectError error
	}{
		{
			name: "Success",
			refs: []telemetryv1beta1.SecretKeyRef{
				{Name: "my-secret1", Namespace: "default", Key: "myKey1"},
				{Name: "my-secret2", Namespace: "default", Key: "myKey2"},
			},
			expectError: nil,
		},
		{
			name: "SecretNameNotPresent",
			refs: []telemetryv1beta1.SecretKeyRef{
				{Name: "my-secret1", Namespace: "default", Key: "myKey1"},
				{Name: "notExistent", Namespace: "default", Key: "myKey2"},
			},
			expectError: secretref.ErrSecretRefNotFound,
		},
		{
			name: "SecretNamespaceNotPresent",
			refs: []telemetryv1beta1.SecretKeyRef{
				{Name: "my-secret1", Namespace: "default", Key: "myKey1"},
				{Name: "my-secret2", Namespace: "notExistent", Key: "myKey2"},
			},
			expectError: secretref.ErrSecretRefNotFound,
		},
		{
			name: "SecretKeyNotPresent",
			refs: []telemetryv1beta1.SecretKeyRef{
				{Name: "my-secret1", Namespace: "default", Key: "myKey1"},
				{Name: "my-secret2", Namespace: "default", Key: "notExistent"},
			},
			expectError: secretref.ErrSecretKeyNotFound,
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
			err := secretRefValidator.validate(t.Context(), test.refs)
			require.ErrorIs(t, err, test.expectError)
		})
	}
}
