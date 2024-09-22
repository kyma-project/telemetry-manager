package webhookcert

import (
	"context"
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/k8sutils"
)

// ensureLogPipelineWebhookConfigs creates or updates the ValidatingWebhookConfiguration for the LogPipeline resources.
// additionally it patches the conversion webhook configuration with the CA bundle.
func ensureLogPipelineWebhookConfigs(ctx context.Context, c client.Client, caBundle []byte, config Config) error {
	validatingWebhookConfig := makeValidatingWebhookConfig(caBundle, config)
	if err := k8sutils.CreateOrUpdateValidatingWebhookConfiguration(ctx, c, &validatingWebhookConfig); err != nil {
		return fmt.Errorf("failed to create or update validating webhook configuration: %w", err)
	}

	if err := patchConversionWebhookConfig(ctx, c, caBundle); err != nil {
		return fmt.Errorf("failed to patch conversion webhook configuration: %w", err)
	}

	return nil
}

func makeValidatingWebhookConfig(certificate []byte, config Config) admissionregistrationv1.ValidatingWebhookConfiguration {
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
	servicePort := int32(443)
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
						Port:      &servicePort,
						Path:      &logPipelinePath,
					},
					CABundle: certificate,
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
						Port:      &servicePort,
						Path:      &logParserPath,
					},
					CABundle: certificate,
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

func patchConversionWebhookConfig(ctx context.Context, c client.Client, caBundle []byte) error {
	var logPipelineCRD apiextensionsv1.CustomResourceDefinition
	if err := c.Get(ctx, types.NamespacedName{Name: "logpipelines.telemetry.kyma-project.io"}, &logPipelineCRD); err != nil {
		return fmt.Errorf("failed to get logpipelines CRD: %w", err)
	}

	patch := client.MergeFrom(logPipelineCRD.DeepCopy())

	conversion := logPipelineCRD.Spec.Conversion
	if conversion != nil && conversion.Webhook != nil && conversion.Webhook.ClientConfig != nil {
		conversion.Webhook.ClientConfig.CABundle = caBundle
	}

	return c.Patch(ctx, &logPipelineCRD, patch)
}
