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

	"github.com/kyma-project/telemetry-manager/internal/k8sutils"
)

const (
	webhookServicePort int32 = 443
)

// ensureLogPipelineWebhookConfigs creates or updates the ValidatingWebhookConfiguration for the LogPipeline resources.
// additionally it patches the conversion webhook configuration with the CA bundle.
func ensureLogPipelineWebhookConfigs(ctx context.Context, c client.Client, caBundle []byte, config Config) error {
	validatingWebhookConfig := makeValidatingWebhookConfig(caBundle, config)
	if err := k8sutils.CreateOrUpdateValidatingWebhookConfiguration(ctx, c, &validatingWebhookConfig); err != nil {
		return fmt.Errorf("failed to create or update validating webhook configuration: %w", err)
	}

	conversionWebhookConfig := makeConversionWebhookConfig(caBundle, config)
	if err := patchConversionWebhookConfig(ctx, c, conversionWebhookConfig); err != nil {
		return fmt.Errorf("failed to patch conversion webhook configuration: %w", err)
	}

	return nil
}

func makeValidatingWebhookConfig(caBundle []byte, config Config) admissionregistrationv1.ValidatingWebhookConfiguration {
	logPipelinePath := "/validate-logpipeline"
	logParserPath := "/validate-logparser"
	failurePolicy := admissionregistrationv1.Fail
	matchPolicy := admissionregistrationv1.Exact
	sideEffects := admissionregistrationv1.SideEffectClassNone
	operations := []admissionregistrationv1.OperationType{
		admissionregistrationv1.Create,
		admissionregistrationv1.Update,
	}
	apiGroups := []string{"telemetry.kyma-project.io"}
	apiVersions := []string{"v1alpha1"}
	scope := admissionregistrationv1.AllScopes
	timeout := int32(15)
	labels := map[string]string{
		"control-plane":              "telemetry-manager",
		"app.kubernetes.io/instance": "telemetry",
		"app.kubernetes.io/name":     "manager",
		"kyma-project.io/component":  "controller",
	}

	return admissionregistrationv1.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:   config.WebhookName.Name,
			Labels: labels,
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				AdmissionReviewVersions: []string{"v1beta1", "v1"},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Name:      config.ServiceName.Name,
						Namespace: config.ServiceName.Namespace,
						Port:      ptr.To(webhookServicePort),
						Path:      &logPipelinePath,
					},
					CABundle: caBundle,
				},
				FailurePolicy:  &failurePolicy,
				MatchPolicy:    &matchPolicy,
				Name:           "validation.logpipelines.telemetry.kyma-project.io",
				SideEffects:    &sideEffects,
				TimeoutSeconds: &timeout,
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: operations,
						Rule: admissionregistrationv1.Rule{
							APIGroups:   apiGroups,
							APIVersions: apiVersions,
							Scope:       &scope,
							Resources:   []string{"logpipelines"},
						},
					},
				},
			},
			{
				AdmissionReviewVersions: []string{"v1beta1", "v1"},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Name:      config.ServiceName.Name,
						Namespace: config.ServiceName.Namespace,
						Port:      ptr.To(webhookServicePort),
						Path:      &logParserPath,
					},
					CABundle: caBundle,
				},
				FailurePolicy:  &failurePolicy,
				MatchPolicy:    &matchPolicy,
				Name:           "validation.logparsers.telemetry.kyma-project.io",
				SideEffects:    &sideEffects,
				TimeoutSeconds: &timeout,
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: operations,
						Rule: admissionregistrationv1.Rule{
							APIGroups:   apiGroups,
							APIVersions: apiVersions,
							Scope:       &scope,
							Resources:   []string{"logparsers"},
						},
					},
				},
			},
		},
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
