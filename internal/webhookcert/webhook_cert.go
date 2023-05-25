package webhookcert

import (
	"context"
	"fmt"
	"os"
	"path"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/kubernetes"
)

const (
	certFile = "tls.crt"
	keyFile  = "tls.key"
)

type Config struct {
	CertDir      string
	ServiceName  types.NamespacedName
	CASecretName types.NamespacedName
	WebhookName  types.NamespacedName
}

func EnsureCertificate(ctx context.Context, client client.Client, certConfig Config) error {
	caCertPEM, caKeyPEM, err := newCACertProvider(client).provideCert(ctx, certConfig.CASecretName)
	if err != nil {
		return fmt.Errorf("failed to get or create ca cert/key: %w", err)
	}

	host, alternativeDNSNames := dnsNames(certConfig.ServiceName)
	var serverCertPEM, serverKeyPEM []byte
	serverCertPEM, serverKeyPEM, err = generateServerCertKey(host, alternativeDNSNames, caCertPEM, caKeyPEM)
	if err != nil {
		return fmt.Errorf("failed to generateCert server cert: %w", err)
	}

	if err = writeFiles(serverCertPEM, serverKeyPEM, certConfig.CertDir); err != nil {
		return fmt.Errorf("failed to write files %w", err)
	}

	validatingWebhookConfig := makeValidatingWebhookConfig(caCertPEM, certConfig)
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

func writeFiles(certPEM, keyPEM []byte, certDir string) error {
	if err := os.WriteFile(path.Join(certDir, certFile), certPEM, 0600); err != nil {
		return err
	}
	return os.WriteFile(path.Join(certDir, keyFile), keyPEM, 0600)
}
