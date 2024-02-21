package webhookcert

import (
	"context"
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/k8sutils"
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

	validatingWebhookConfig := makeValidatingWebhookConfig(caCertPEM, config)
	return k8sutils.CreateOrUpdateValidatingWebhookConfiguration(ctx, client, &validatingWebhookConfig)
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
