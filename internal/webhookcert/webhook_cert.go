package webhookcert

import (
	"context"
	"fmt"
	"os"
	"path"

	v1 "k8s.io/api/admissionregistration/v1"
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
	caCertFile       = "ca.crt"
	caKeyFile        = "ca.key"
)

func EnsureCertificate(ctx context.Context, client client.Client, webhookService types.NamespacedName, certDir string) (*v1.ValidatingWebhookConfiguration, *corev1.Secret, error) {
	var caSecret *corev1.Secret
	caCertPEM, caKeyPEM, caSecret, err := ensureCACertKey(ctx, client, webhookService.Namespace)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get or create ca cert/key: %w", err)
	}

	host, alternativeDNSNames := dnsNames(webhookService)
	var serverCertPEM, serverKeyPEM []byte
	serverCertPEM, serverKeyPEM, err = generateServerCertKey(host, alternativeDNSNames, caCertPEM, caKeyPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate server cert: %w", err)
	}

	if err = writeFiles(serverCertPEM, serverKeyPEM, certDir); err != nil {
		return nil, nil, fmt.Errorf("failed to write files %w", err)
	}

	validatingWebhookConfig := makeValidatingWebhookConfig(caCertPEM, webhookService)
	return &validatingWebhookConfig, caSecret, kubernetes.CreateOrUpdateValidatingWebhookConfiguration(ctx, client, &validatingWebhookConfig)
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

func ensureCACertKey(ctx context.Context, client client.Client, caCertNamespace string) ([]byte, []byte, *corev1.Secret, error) {
	var caCertPEM, caKeyPEM []byte
	caSecretKey := types.NamespacedName{Name: caCertSecretName, Namespace: caCertNamespace}
	var caSecret corev1.Secret
	err := client.Get(ctx, caSecretKey, &caSecret)
	if err == nil {
		logf.FromContext(ctx).Info("Found existing CA cert/key",
			"secretName", caCertSecretName,
			"secretNamespace", caCertNamespace)

		if val, found := caSecret.Data[caCertFile]; found {
			caCertPEM = val
		} else {
			return nil, nil, nil, fmt.Errorf("key not found: %v", certFile)
		}

		if val, found := caSecret.Data[caKeyFile]; found {
			caKeyPEM = val
		} else {
			return nil, nil, nil, fmt.Errorf("key not found: %v", keyFile)
		}
	} else {
		if !apierrors.IsNotFound(err) {
			return nil, nil, nil, fmt.Errorf("failed to find ca cert secret: %w", err)
		}

		caCertPEM, caKeyPEM, err = generateCACertKey()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to generate ca cert: %w", err)
		}

		logf.FromContext(ctx).Info("Generated new CA cert/key")
		newSecret := makeCaSecret(caCertPEM, caKeyPEM, caSecretKey)
		if err = client.Create(ctx, &newSecret); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to create ca cert secret: %w", err)
		}
	}

	return caCertPEM, caKeyPEM, &caSecret, nil
}

func writeFiles(certPEM, keyPEM []byte, certDir string) error {
	if err := os.WriteFile(path.Join(certDir, certFile), certPEM, 0600); err != nil {
		return err
	}
	return os.WriteFile(path.Join(certDir, keyFile), keyPEM, 0600)
}
