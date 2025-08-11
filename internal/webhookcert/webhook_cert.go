package webhookcert

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Config struct {
	rsaKeySize            int
	CertDir               string
	ServiceName           types.NamespacedName
	CASecretName          types.NamespacedName
	ValidatingWebhookName types.NamespacedName
	MutatingWebhookName   types.NamespacedName
}

func NewWebhookCertConfig(certDir string, serviceName, caSecretName, validatingWebhookName, mutatingWebhookName types.NamespacedName) Config {
	return Config{
		rsaKeySize:            rsaKeySize,
		CertDir:               certDir,
		ServiceName:           serviceName,
		CASecretName:          caSecretName,
		ValidatingWebhookName: validatingWebhookName,
		MutatingWebhookName:   mutatingWebhookName,
	}
}

func EnsureCertificate(ctx context.Context, client client.Client, config Config) error {
	caCertPEM, caKeyPEM, err := newCACertProvider(client, config.rsaKeySize).provideCert(ctx, config.CASecretName)
	if err != nil {
		return fmt.Errorf("failed to provide ca cert/key: %w", err)
	}

	host, alternativeDNSNames := dnsNames(config.ServiceName)

	_, _, err = newServerCertProvider(config.CertDir, config.rsaKeySize).provideCert(ctx, serverCertConfig{
		host:                host,
		alternativeDNSNames: alternativeDNSNames,
		caCertPEM:           caCertPEM,
		caKeyPEM:            caKeyPEM,
	})
	if err != nil {
		return fmt.Errorf("failed to provide server cert/key: %w", err)
	}

	return applyWebhookConfigResources(ctx, client, caCertPEM, config)
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
