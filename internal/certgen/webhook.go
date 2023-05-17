package certgen

import (
	"context"
	"fmt"
	"os"
	"path"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/cert"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/kubernetes"
	"github.com/kyma-project/telemetry-manager/internal/resources/webhook"
)

var (
	webhookCertSecret = "telemetry-webhook-cert"
	certFile          = "tls.crt"
	keyFile           = "tls.key"
)

func generateCert(serviceName, namespace string) ([]byte, []byte, error) {
	cn := fmt.Sprintf("%s.%s.svc", serviceName, namespace)
	names := []string{
		serviceName,
		fmt.Sprintf("%s.%s", serviceName, namespace),
		fmt.Sprintf("%s.cluster.local", cn),
	}
	return cert.GenerateSelfSignedCertKey(cn, nil, names)
}

func EnsureValidatingWebhookConfig(ctx context.Context, client client.Client, webhookService types.NamespacedName, certDir string) error {
	secretKey := types.NamespacedName{
		Name:      webhookCertSecret,
		Namespace: webhookService.Namespace,
	}
	var secret corev1.Secret
	err := client.Get(ctx, secretKey, &secret)

	var certificate, key []byte
	if err == nil {
		// TODO: check if certificate is still valid
		certificate = secret.Data[certFile]
		key = secret.Data[keyFile]
	} else {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get secret: %w", err)
		}

		certificate, key, err = generateCert(webhookService.Name, webhookService.Namespace)
		if err != nil {
			return fmt.Errorf("failed to generate certificate: %w", err)
		}

		newSecret := webhook.MakeCertificateSecret(certificate, key, secretKey)
		if err = client.Create(ctx, &newSecret); err != nil {
			return fmt.Errorf("failed to create secret: %w", err)
		}
	}

	if err = os.WriteFile(path.Join(certDir, certFile), certificate, 0600); err != nil {
		return fmt.Errorf("failed to write %v: %w", certFile, err)
	}

	if err = os.WriteFile(path.Join(certDir, keyFile), key, 0600); err != nil {
		return fmt.Errorf("failed to write %v: %w", keyFile, err)
	}

	validatingWebhookConfig := webhook.MakeValidatingWebhookConfig(certificate, webhookService)
	return kubernetes.CreateOrUpdateValidatingWebhookConfiguration(ctx, client, &validatingWebhookConfig)
}
