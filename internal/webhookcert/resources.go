package webhookcert

import (
	"context"
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	webhookServicePort int32 = 443
)

// applyWebhookConfigResources applies the following webhook configurations:
// 1- Updates validating webhook configuration with the provided CA bundle.
// 2- Updates LogPipeline CRD with conversion webhook configuration.
// 3- Updates mutating webhook configuration with the provided CA bundle.
func applyWebhookConfigResources(ctx context.Context, c client.Client, caBundle []byte, config Config) error {
	if err := updateValidatingWebhookConfig(ctx, c, caBundle, config); err != nil {
		return fmt.Errorf("failed to update validating webhook with CA bundle: %w", err)
	}

	if err := updateMutatingWebhookConfig(ctx, c, caBundle, config); err != nil {
		return fmt.Errorf("failed to patch mutating webhook configuration: %w", err)
	}

	conversionWebhookConfig := makeConversionWebhookConfig(caBundle, config)
	if err := updateLogPipelineCRDWithConversionWebhookConfig(ctx, c, conversionWebhookConfig); err != nil {
		return fmt.Errorf("failed to update LogPipeline CRD with conversion webhook configuration: %w", err)
	}

	return nil
}

func updateValidatingWebhookConfig(ctx context.Context, c client.Client, caBundle []byte, config Config) error {
	var validatingWebhookConfig admissionregistrationv1.ValidatingWebhookConfiguration
	if err := c.Get(ctx, config.ValidatingWebhookName, &validatingWebhookConfig); err != nil {
		return fmt.Errorf("failed to get validating webhook configuration: %w", err)
	}

	validatingWebhookConfig.Webhooks[0].ClientConfig.CABundle = caBundle
	validatingWebhookConfig.Webhooks[1].ClientConfig.CABundle = caBundle

	return c.Update(ctx, &validatingWebhookConfig)
}

func makeConversionWebhookConfig(caBundle []byte, config Config) apiextensionsv1.CustomResourceConversion {
	return apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.WebhookConverter,
		Webhook: &apiextensionsv1.WebhookConversion{
			ClientConfig: &apiextensionsv1.WebhookClientConfig{
				Service: &apiextensionsv1.ServiceReference{
					Namespace: config.ServiceName.Namespace,
					Name:      config.ServiceName.Name,
					Path:      ptr.To("/convert"),
					Port:      ptr.To(webhookServicePort),
				},
				CABundle: caBundle,
			},
			ConversionReviewVersions: []string{"v1"},
		},
	}
}

func updateLogPipelineCRDWithConversionWebhookConfig(ctx context.Context, c client.Client, conversion apiextensionsv1.CustomResourceConversion) error {
	var logPipelineCRD apiextensionsv1.CustomResourceDefinition
	if err := c.Get(ctx, types.NamespacedName{Name: "logpipelines.telemetry.kyma-project.io"}, &logPipelineCRD); err != nil {
		return fmt.Errorf("failed to get logpipelines CRD: %w", err)
	}

	logPipelineCRD.Spec.Conversion = &conversion

	return c.Update(ctx, &logPipelineCRD)
}

func updateMutatingWebhookConfig(ctx context.Context, c client.Client, caBundle []byte, config Config) error {
	var mutatingWebhook admissionregistrationv1.MutatingWebhookConfiguration
	if err := c.Get(ctx, config.MutatingWebhookName, &mutatingWebhook); err != nil {
		return fmt.Errorf("failed to get mutating webhook: %w", err)
	}

	for i := range len(mutatingWebhook.Webhooks) {
		mutatingWebhook.Webhooks[i].ClientConfig.CABundle = caBundle
	}

	return c.Update(ctx, &mutatingWebhook)
}
