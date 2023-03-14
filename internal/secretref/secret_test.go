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

func TestFetchSecretValue(t *testing.T) {
	data := map[string][]byte{
		"myKey": []byte("myValue"),
	}
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "default",
		},
		Data: data,
	}
	client := fake.NewClientBuilder().WithObjects(&secret).Build()

	value := telemetryv1alpha1.ValueType{
		ValueFrom: &telemetryv1alpha1.ValueFromSource{
			SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
				Name:      "my-secret",
				Namespace: "default",
				Key:       "myKey",
			},
		},
	}

	fetchedData, err := fetchSecretValue(context.TODO(), client, value)

	require.Nil(t, err)
	require.Equal(t, string(fetchedData), "myValue")
}

func TestFetchValueFromNonExistingSecret(t *testing.T) {
	client := fake.NewClientBuilder().Build()

	value := telemetryv1alpha1.ValueType{
		ValueFrom: &telemetryv1alpha1.ValueFromSource{
			SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
				Name:      "my-secret",
				Namespace: "default",
				Key:       "myKey",
			},
		},
	}

	_, err := fetchSecretValue(context.TODO(), client, value)
	require.Error(t, err)
}

func TestFetchValueFromNonExistingKey(t *testing.T) {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "default",
		},
	}
	client := fake.NewClientBuilder().WithObjects(&secret).Build()

	value := telemetryv1alpha1.ValueType{
		ValueFrom: &telemetryv1alpha1.ValueFromSource{
			SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
				Name:      "my-secret",
				Namespace: "default",
				Key:       "myKey",
			},
		},
	}

	_, err := fetchSecretValue(context.TODO(), client, value)
	require.Error(t, err)
}
