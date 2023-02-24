package setup

import (
	"context"
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/kubernetes"
	"github.com/kyma-project/telemetry-manager/internal/resources/webhook"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/cert"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GenerateCert(serviceName, namespace string) ([]byte, []byte, error) {
	cn := fmt.Sprintf("%s.%s.svc", serviceName, namespace)
	names := []string{
		serviceName,
		fmt.Sprintf("%s.%s", serviceName, namespace),
		fmt.Sprintf("%s.cluster.local", cn),
	}
	return cert.GenerateSelfSignedCertKey(cn, nil, names)
}

func EnsureValidatingWebhookConfig(client client.Client, webhookService types.NamespacedName, certificate []byte) error {
	ctx := context.Background()
	validatingWebhookConfig := webhook.MakeValidatingWebhookConfig(certificate, webhookService)
	return kubernetes.CreateOrUpdateValidatingWebhookConfiguration(ctx, client, &validatingWebhookConfig)
}
