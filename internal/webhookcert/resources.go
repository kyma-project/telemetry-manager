package webhookcert

import (
	"context"
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
)

const (
	webhookServicePort int32 = 443
)

// applyWebhookConfigResources creates or updates a ValidatingWebhookConfiguration for the LogPipeline/LogParser resources.
// additionally it patches a LogPipeline conversion webhook configuration.
func applyWebhookConfigResources(ctx context.Context, c client.Client, caBundle []byte, config Config) error {
	validatingWebhookConfig := makeValidatingWebhookConfig(caBundle, config)
	if err := k8sutils.CreateOrUpdateValidatingWebhookConfiguration(ctx, c, &validatingWebhookConfig); err != nil {
		return fmt.Errorf("failed to create or update validating webhook configuration: %w", err)
	}

	if err := patchMutatingWebhook(ctx, c, caBundle, config); err != nil {
		return fmt.Errorf("failed to patch mutating webhook configuration: %w", err)
	}

	conversionWebhookConfig := makeConversionWebhookConfig(caBundle, config)
	if err := patchConversionWebhookConfig(ctx, c, conversionWebhookConfig); err != nil {
		return fmt.Errorf("failed to patch conversion webhook configuration: %w", err)
	}

	return nil
}

func makeValidatingWebhookConfig(caBundle []byte, config Config) admissionregistrationv1.ValidatingWebhookConfiguration {
	apiGroups := []string{"telemetry.kyma-project.io"}
	apiVersions := []string{"v1alpha1"}
	webhookTimeout := int32(15) //nolint:mnd // 15 seconds
	labels := map[string]string{
		"control-plane":                    "telemetry-manager",
		"app.kubernetes.io/instance":       "telemetry",
		"app.kubernetes.io/validatingName": "manager",
		"kyma-project.io/component":        "controller",
	}

	createWebhook := func(name, path string, resources []string) admissionregistrationv1.ValidatingWebhook {
		return admissionregistrationv1.ValidatingWebhook{
			AdmissionReviewVersions: []string{"v1beta1", "v1"},
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				Service: &admissionregistrationv1.ServiceReference{
					Name:      config.ServiceName.Name,
					Namespace: config.ServiceName.Namespace,
					Port:      ptr.To(webhookServicePort),
					Path:      &path,
				},
				CABundle: caBundle,
			},
			FailurePolicy:  ptr.To(admissionregistrationv1.Fail),
			MatchPolicy:    ptr.To(admissionregistrationv1.Exact),
			Name:           name,
			SideEffects:    ptr.To(admissionregistrationv1.SideEffectClassNone),
			TimeoutSeconds: &webhookTimeout,
			Rules: []admissionregistrationv1.RuleWithOperations{
				{
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
					},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   apiGroups,
						APIVersions: apiVersions,
						Scope:       ptr.To(admissionregistrationv1.AllScopes),
						Resources:   resources,
					},
				},
			},
		}
	}

	webhooks := []admissionregistrationv1.ValidatingWebhook{
		createWebhook("validation.logpipelines.telemetry.kyma-project.io", "/validate-logpipeline", []string{"logpipelines"}),
		createWebhook("validation.logparsers.telemetry.kyma-project.io", "/validate-logparser", []string{"logparsers"}),
	}

	return admissionregistrationv1.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:   config.ValidatingWebhookName.Name,
			Labels: labels,
		},
		Webhooks: webhooks,
	}
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

func patchConversionWebhookConfig(ctx context.Context, c client.Client, conversion apiextensionsv1.CustomResourceConversion) error {
	var logPipelineCRD apiextensionsv1.CustomResourceDefinition
	if err := c.Get(ctx, types.NamespacedName{Name: "logpipelines.telemetry.kyma-project.io"}, &logPipelineCRD); err != nil {
		return fmt.Errorf("failed to get logpipelines CRD: %w", err)
	}

	patch := client.MergeFrom(logPipelineCRD.DeepCopy())

	logPipelineCRD.Spec.Conversion = &conversion

	return c.Patch(ctx, &logPipelineCRD, patch)
}

func patchMutatingWebhook(ctx context.Context, c client.Client, caBundle []byte, config Config) error {
	var mutatingWebhook admissionregistrationv1.MutatingWebhookConfiguration
	if err := c.Get(ctx, config.MutatingWebhookName, &mutatingWebhook); err != nil {
		return fmt.Errorf("failed to get mutating webhook: %w", err)
	}

	patch := client.MergeFrom(mutatingWebhook.DeepCopy())

	for _, mWebhook := range mutatingWebhook.Webhooks {
		mWebhook.ClientConfig.CABundle = caBundle
	}

	return c.Patch(ctx, &mutatingWebhook, patch)
}
