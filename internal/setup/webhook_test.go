package setup

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

var (
	webhookService = types.NamespacedName{
		Name:      "telemetry-operator-webhook",
		Namespace: "telemetry-system",
	}
	name   = "validation.webhook.telemetry.kyma-project.io"
	labels = map[string]string{
		"control-plane":              "telemetry-operator",
		"app.kubernetes.io/instance": "telemetry",
		"app.kubernetes.io/name":     "operator",
		"kyma-project.io/component":  "controller",
	}
)

func TestEnsureValidatingWebhookConfig(t *testing.T) {
	client := fake.NewClientBuilder().Build()

	err := EnsureValidatingWebhookConfig(client, webhookService, []byte("123"))
	require.NoError(t, err)

	var validatingWebhookConfiguration admissionregistrationv1.ValidatingWebhookConfiguration
	key := types.NamespacedName{
		Name: name,
	}

	err = client.Get(context.Background(), key, &validatingWebhookConfiguration)
	require.NoError(t, err)

	require.Equal(t, name, validatingWebhookConfiguration.Name)
	require.Equal(t, labels, validatingWebhookConfiguration.Labels)

	require.Equal(t, 2, len(validatingWebhookConfiguration.Webhooks))

	require.Equal(t, int32(15), *validatingWebhookConfiguration.Webhooks[0].TimeoutSeconds)
	require.Equal(t, int32(15), *validatingWebhookConfiguration.Webhooks[1].TimeoutSeconds)

	require.Equal(t, []byte("123"), validatingWebhookConfiguration.Webhooks[0].ClientConfig.CABundle)
	require.Equal(t, []byte("123"), validatingWebhookConfiguration.Webhooks[1].ClientConfig.CABundle)

	require.Equal(t, webhookService.Name, validatingWebhookConfiguration.Webhooks[0].ClientConfig.Service.Name)
	require.Equal(t, webhookService.Name, validatingWebhookConfiguration.Webhooks[1].ClientConfig.Service.Name)

	require.Equal(t, webhookService.Namespace, validatingWebhookConfiguration.Webhooks[0].ClientConfig.Service.Namespace)
	require.Equal(t, webhookService.Namespace, validatingWebhookConfiguration.Webhooks[1].ClientConfig.Service.Namespace)

	require.Equal(t, int32(443), *validatingWebhookConfiguration.Webhooks[0].ClientConfig.Service.Port)
	require.Equal(t, int32(443), *validatingWebhookConfiguration.Webhooks[1].ClientConfig.Service.Port)

	require.Equal(t, "/validate-logpipeline", *validatingWebhookConfiguration.Webhooks[0].ClientConfig.Service.Path)
	require.Equal(t, "/validate-logparser", *validatingWebhookConfiguration.Webhooks[1].ClientConfig.Service.Path)

	require.Contains(t, validatingWebhookConfiguration.Webhooks[0].Rules[0].APIGroups, "telemetry.kyma-project.io")
	require.Contains(t, validatingWebhookConfiguration.Webhooks[1].Rules[0].APIGroups, "telemetry.kyma-project.io")

	require.Contains(t, validatingWebhookConfiguration.Webhooks[0].Rules[0].APIVersions, "v1alpha1")
	require.Contains(t, validatingWebhookConfiguration.Webhooks[1].Rules[0].APIVersions, "v1alpha1")

	require.Contains(t, validatingWebhookConfiguration.Webhooks[0].Rules[0].Resources, "logpipelines")
	require.Contains(t, validatingWebhookConfiguration.Webhooks[1].Rules[0].Resources, "logparsers")

}

func TestUpdateWebhookCertificate(t *testing.T) {
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
	certificate := []byte("123")

	initialValidatingWebhookConfiguration := &admissionregistrationv1.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
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
	client := fake.NewClientBuilder().WithObjects(initialValidatingWebhookConfiguration).Build()
	newCertificate := []byte("asdf")

	err := EnsureValidatingWebhookConfig(client, webhookService, newCertificate)
	require.NoError(t, err)

	var updatedValidatingWebhookConfiguration admissionregistrationv1.ValidatingWebhookConfiguration
	key := types.NamespacedName{
		Name: name,
	}

	err = client.Get(context.Background(), key, &updatedValidatingWebhookConfiguration)
	require.NoError(t, err)

	require.Equal(t, newCertificate, updatedValidatingWebhookConfiguration.Webhooks[0].ClientConfig.CABundle)
	require.Equal(t, newCertificate, updatedValidatingWebhookConfiguration.Webhooks[1].ClientConfig.CABundle)
}
