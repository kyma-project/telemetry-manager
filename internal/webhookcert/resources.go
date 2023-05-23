package webhookcert

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func makeCaSecret(certificate []byte, key []byte, name types.NamespacedName) corev1.Secret {
	return corev1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
		Data: map[string][]byte{
			caCertFile: certificate,
			caKeyFile:  key,
		},
	}
}

func makeValidatingWebhookConfig(certificate []byte, webhookService types.NamespacedName) admissionregistrationv1.ValidatingWebhookConfiguration {
	webhookName := "validation.webhook.telemetry.kyma-project.io"
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
		"control-plane":              "telemetry-operator",
		"app.kubernetes.io/instance": "telemetry",
		"app.kubernetes.io/name":     "operator",
		"kyma-project.io/component":  "controller",
	}

	return admissionregistrationv1.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:   webhookName,
			Labels: labels,
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				AdmissionReviewVersions: []string{"v1beta1", "v1"},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Name:      webhookService.Name,
						Namespace: webhookService.Namespace,
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
						Name:      webhookService.Name,
						Namespace: webhookService.Namespace,
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
