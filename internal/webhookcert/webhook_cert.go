package webhookcert

import (
	"context"
	"fmt"
	"os"
	"path"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/telemetry-manager/internal/kubernetes"
)

var (
	caCertSecretName = "telemetry-webhook-cert"
	certFile         = "tls.crt"
	keyFile          = "tls.key"
)

func EnsureCertificate(ctx context.Context, client client.Client, webhookService types.NamespacedName, certDir string) error {
	caCertPEM, caKeyPEM, err := getOrCreateCACertKey(ctx, client, webhookService.Namespace)
	if err != nil {
		return fmt.Errorf("failed to get or create ca cert/key: %w", err)
	}

	var serverCertPEM, serverKeyPEM []byte
	host, alternativeDNSNames := dnsNames(webhookService)
	serverCertPEM, serverKeyPEM, err = generateServerCertKey(host, alternativeDNSNames, caCertPEM, caKeyPEM)
	if err != nil {
		return fmt.Errorf("failed to generate server cert: %w", err)
	}

	if err = os.WriteFile(path.Join(certDir, certFile), serverCertPEM, 0600); err != nil {
		return fmt.Errorf("failed to write %v: %w", certFile, err)
	}

	if err = os.WriteFile(path.Join(certDir, keyFile), serverKeyPEM, 0600); err != nil {
		return fmt.Errorf("failed to write %v: %w", keyFile, err)
	}

	validatingWebhookConfig := makeValidatingWebhookConfig(caCertPEM, webhookService)
	return kubernetes.CreateOrUpdateValidatingWebhookConfiguration(ctx, client, &validatingWebhookConfig)
}

func dnsNames(webhookService types.NamespacedName) (host string, alternativeDNSNames []string) {
	host = fmt.Sprintf("%s.%s.svc", webhookService.Name, webhookService.Namespace)
	alternativeDNSNames = []string{
		webhookService.Name,
		fmt.Sprintf("%s.%s", webhookService.Name, webhookService.Namespace),
		fmt.Sprintf("%s.cluster.local", host),
	}
	return
}

func getOrCreateCACertKey(ctx context.Context, client client.Client, caCertNamespace string) ([]byte, []byte, error) {
	var caCertPEM, caKeyPEM []byte
	caSecretKey := types.NamespacedName{Name: caCertSecretName, Namespace: caCertNamespace}
	var caSecret corev1.Secret
	err := client.Get(ctx, caSecretKey, &caSecret)
	if err == nil {
		logf.FromContext(ctx).Info("Found existing CA cert/key",
			"secretName", caCertSecretName,
			"secretNamespace", caCertNamespace)

		if val, found := caSecret.Data[certFile]; found {
			caCertPEM = val
		} else {
			return nil, nil, fmt.Errorf("key not found : %v", certFile)
		}

		if val, found := caSecret.Data[keyFile]; found {
			caKeyPEM = val
		} else {
			return nil, nil, fmt.Errorf("key not found : %v", keyFile)
		}
	} else {
		if !apierrors.IsNotFound(err) {
			return nil, nil, fmt.Errorf("failed to find ca cert secret: %w", err)
		}

		caCertPEM, caKeyPEM, err = generateCACertKey()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to generate ca cert: %w", err)
		}

		logf.FromContext(ctx).Info("Generated new CA cert/key")
		newSecret := makeCertSecret(caCertPEM, caKeyPEM, caSecretKey)
		if err = client.Create(ctx, &newSecret); err != nil {
			return nil, nil, fmt.Errorf("failed to create ca cert secret: %w", err)
		}
	}

	return caCertPEM, caKeyPEM, nil
}
