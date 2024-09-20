package webhookcert

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Config struct {
	CertDir      string
	ServiceName  types.NamespacedName
	CASecretName types.NamespacedName
	WebhookName  types.NamespacedName
}

func EnsureCertificate(ctx context.Context, client client.Client, config Config) error {
	caCertPEM, caKeyPEM, err := newCACertProvider(client).provideCert(ctx, config.CASecretName)
	if err != nil {
		return fmt.Errorf("failed to provider ca cert/key: %w", err)
	}

	host, alternativeDNSNames := dnsNames(config.ServiceName)
	_, _, err = newServerCertProvider(config.CertDir).provideCert(ctx, serverCertConfig{
		host:                host,
		alternativeDNSNames: alternativeDNSNames,
		caCertPEM:           caCertPEM,
		caKeyPEM:            caKeyPEM,
	})
	if err != nil {
		return fmt.Errorf("failed to provider server cert/key: %w", err)
	}

	return ensureLogPipelineWebhookConfigs(ctx, client, caCertPEM, config)
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
