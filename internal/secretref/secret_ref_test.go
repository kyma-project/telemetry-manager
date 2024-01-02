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

func TestValidateAndSanitizeTLSSecret(t *testing.T) {
	key := "-----BEGIN PUBLIC KEY-----\\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEArvbfKwQcdFZZwK5CKpo1\nHHYfp6hY3ucOITQlgMcGWIZi4qJXck1RkwKGf4rz/6trKioKCyCrvvS2LtZO+tk+\ncboVlH1vgQ+JlULqBejcSZTltVILbU4V5Qw9PzFo0OAi+Poi4Za+zitz6WkMJWpN\nJ5YDuFOnitr045E24GWF0Z0brFuzkn2A5XFEqlO065sCbl77oOkKG5j1mEnamkxu\n3C6hfuVaMtcNiSoXHkNQxr1GfKlRTblGYr8rXRLBqZ8WkxFEStLHIK6GF6T/49Vv\nQldniWBiP2DBq7bjM9EkUz+84f2clLN8bVHKZxJAErhC3exilkPiZYi5x5U6gYfy\n3wIDAQAB\\n-----END PUBLIC KEY-----\n"
	cert := "-----BEGIN CERTIFICATE-----\\nMIIDLzCCAhegAwIBAgIUTDC2e9uCi0ggzWiI7XkkXNWmMY0wDQYJKoZIhvcNAQEL\nBQAwJzELMAkGA1UEBhMCVVMxGDAWBgNVBAMMD0V4YW1wbGUtZm9vLWJhcjAeFw0y\nMzEyMjgxMzUwNDNaFw0yNjEwMTcxMzUwNDNaMCcxCzAJBgNVBAYTAlVTMRgwFgYD\nVQQDDA9FeGFtcGxlLWZvby1iYXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK\nAoIBAQCu9t8rBBx0VlnArkIqmjUcdh+nqFje5w4hNCWAxwZYhmLioldyTVGTAoZ/\nivP/q2sqKgoLIKu+9LYu1k762T5xuhWUfW+BD4mVQuoF6NxJlOW1UgttThXlDD0/\nMWjQ4CL4+iLhlr7OK3PpaQwlak0nlgO4U6eK2vTjkTbgZYXRnRusW7OSfYDlcUSq\nU7TrmwJuXvug6QobmPWYSdqaTG7cLqF+5Voy1w2JKhceQ1DGvUZ8qVFNuUZivytd\nEsGpnxaTEURK0scgroYXpP/j1W9CV2eJYGI/YMGrtuMz0SRTP7zh/ZyUs3xtUcpn\nEkASuELd7GKWQ+JliLnHlTqBh/LfAgMBAAGjUzBRMB0GA1UdDgQWBBTYsQEqc5CX\nzjBGv/O04Qd5sOu5QjAfBgNVHSMEGDAWgBTYsQEqc5CXzjBGv/O04Qd5sOu5QjAP\nBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQA1L7IsQ9GTFl9GQMGo\n+JOffZxhR9AiwnCPXTMWF8qYC99F39i946wJIkgJN6wm8Rt46gA65EfJw6YdjdB6\n8kjg3CDRDIFn2QRrP4x8tS4EBu9tUkss/2h0I16MEEB9RV8adjH0lkiPwQwP50uW\nwLlwMHw9KsxA1MATzSmBruzW//gyoJFaBKYsYqYa7VKcEyQqKgiQypBN2O01twF3\ntahLFTIeeD0e4fMe++mwJh8rT5sRpCLmFDIoajmLkjj48P7hvgtLFN+vRTqgqViq\nySngIMt75xyXeTm15o7LrEe4B4HkpWt4CbeUW/44HrCUoItuhyea7baGecLx8VoS\nR3xg\\n-----END CERTIFICATE-----\n"

	expectedKey := "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEArvbfKwQcdFZZwK5CKpo1\nHHYfp6hY3ucOITQlgMcGWIZi4qJXck1RkwKGf4rz/6trKioKCyCrvvS2LtZO+tk+\ncboVlH1vgQ+JlULqBejcSZTltVILbU4V5Qw9PzFo0OAi+Poi4Za+zitz6WkMJWpN\nJ5YDuFOnitr045E24GWF0Z0brFuzkn2A5XFEqlO065sCbl77oOkKG5j1mEnamkxu\n3C6hfuVaMtcNiSoXHkNQxr1GfKlRTblGYr8rXRLBqZ8WkxFEStLHIK6GF6T/49Vv\nQldniWBiP2DBq7bjM9EkUz+84f2clLN8bVHKZxJAErhC3exilkPiZYi5x5U6gYfy\n3wIDAQAB\n-----END PUBLIC KEY-----\n"
	expectedCert := "-----BEGIN CERTIFICATE-----\nMIIDLzCCAhegAwIBAgIUTDC2e9uCi0ggzWiI7XkkXNWmMY0wDQYJKoZIhvcNAQEL\nBQAwJzELMAkGA1UEBhMCVVMxGDAWBgNVBAMMD0V4YW1wbGUtZm9vLWJhcjAeFw0y\nMzEyMjgxMzUwNDNaFw0yNjEwMTcxMzUwNDNaMCcxCzAJBgNVBAYTAlVTMRgwFgYD\nVQQDDA9FeGFtcGxlLWZvby1iYXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK\nAoIBAQCu9t8rBBx0VlnArkIqmjUcdh+nqFje5w4hNCWAxwZYhmLioldyTVGTAoZ/\nivP/q2sqKgoLIKu+9LYu1k762T5xuhWUfW+BD4mVQuoF6NxJlOW1UgttThXlDD0/\nMWjQ4CL4+iLhlr7OK3PpaQwlak0nlgO4U6eK2vTjkTbgZYXRnRusW7OSfYDlcUSq\nU7TrmwJuXvug6QobmPWYSdqaTG7cLqF+5Voy1w2JKhceQ1DGvUZ8qVFNuUZivytd\nEsGpnxaTEURK0scgroYXpP/j1W9CV2eJYGI/YMGrtuMz0SRTP7zh/ZyUs3xtUcpn\nEkASuELd7GKWQ+JliLnHlTqBh/LfAgMBAAGjUzBRMB0GA1UdDgQWBBTYsQEqc5CX\nzjBGv/O04Qd5sOu5QjAfBgNVHSMEGDAWgBTYsQEqc5CXzjBGv/O04Qd5sOu5QjAP\nBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQA1L7IsQ9GTFl9GQMGo\n+JOffZxhR9AiwnCPXTMWF8qYC99F39i946wJIkgJN6wm8Rt46gA65EfJw6YdjdB6\n8kjg3CDRDIFn2QRrP4x8tS4EBu9tUkss/2h0I16MEEB9RV8adjH0lkiPwQwP50uW\nwLlwMHw9KsxA1MATzSmBruzW//gyoJFaBKYsYqYa7VKcEyQqKgiQypBN2O01twF3\ntahLFTIeeD0e4fMe++mwJh8rT5sRpCLmFDIoajmLkjj48P7hvgtLFN+vRTqgqViq\nySngIMt75xyXeTm15o7LrEe4B4HkpWt4CbeUW/44HrCUoItuhyea7baGecLx8VoS\nR3xg\n-----END CERTIFICATE-----\n"

	secretData := make(map[string][]byte)
	secretData["key"] = []byte(key)
	secretData["cert"] = []byte(cert)

	sanitizedSecretData := SanitizeTlSValueOrSecret(secretData, "key", "cert")

	require.Equal(t, string(sanitizedSecretData["cert"]), expectedCert)
	require.Equal(t, string(sanitizedSecretData["key"]), expectedKey)
}
