package webhookcert

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	systemNamespace = "kyma-system"
	webhookService  = types.NamespacedName{
		Name:      "telemetry-manager-webhook",
		Namespace: systemNamespace,
	}
	caBundleSecret = types.NamespacedName{
		Name:      "telemetry-webhook-cert",
		Namespace: systemNamespace,
	}
	validatingWebhookName           = "telemetry-validating-webhook.kyma-project.io"
	validatingWebhookNamespacedName = types.NamespacedName{
		Name: validatingWebhookName,
	}

	mutatingWebhookName           = "telemetry-mutating-webhook.kyma-project.io"
	mutatingWebhookNamespacedName = types.NamespacedName{
		Name: mutatingWebhookName,
	}

	logPipelinesCRD = apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "logpipelines.telemetry.kyma-project.io",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Conversion: &apiextensionsv1.CustomResourceConversion{
				Strategy: apiextensionsv1.WebhookConverter,
				Webhook: &apiextensionsv1.WebhookConversion{
					ClientConfig: &apiextensionsv1.WebhookClientConfig{},
				},
			},
		},
	}

	metricPipelinesCRD = apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "metricpipelines.telemetry.kyma-project.io",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Conversion: &apiextensionsv1.CustomResourceConversion{
				Strategy: apiextensionsv1.WebhookConverter,
				Webhook: &apiextensionsv1.WebhookConversion{
					ClientConfig: &apiextensionsv1.WebhookClientConfig{},
				},
			},
		},
	}

	labels = map[string]string{
		"app.kubernetes.io/component":  "telemetry",
		"app.kubernetes.io/instance":   "telemetry-manager",
		"app.kubernetes.io/managed-by": "kustomize",
		"app.kubernetes.io/name":       "telemetry-manager",
		"app.kubernetes.io/part-of":    "kyma",
		"control-plane":                "telemetry-manager",
	}
	failurePolicy = admissionregistrationv1.Fail
	matchPolicy   = admissionregistrationv1.Exact
	sideEffects   = admissionregistrationv1.SideEffectClassNone
	operations    = []admissionregistrationv1.OperationType{
		admissionregistrationv1.Create,
		admissionregistrationv1.Update,
	}
	apiGroups                      = []string{"telemetry.kyma-project.io"}
	apiVersions                    = []string{"v1alpha1"}
	scope                          = admissionregistrationv1.AllScopes
	servicePort                    = int32(443)
	timeout                        = int32(15)
	validatingWebhookConfiguration = admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   validatingWebhookName,
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
						Path:      ptr.To("/validate-telemetry-kyma-project-io-v1alpha1-logpipeline"),
					},
				},
				FailurePolicy:  &failurePolicy,
				MatchPolicy:    &matchPolicy,
				Name:           "validating-logpipelines.kyma-project.io",
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
		},
	}

	mutatingWebhookConfiguration = admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   mutatingWebhookName,
			Labels: labels,
		},
		Webhooks: []admissionregistrationv1.MutatingWebhook{
			{
				AdmissionReviewVersions: []string{"v1beta1", "v1"},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Name:      webhookService.Name,
						Namespace: webhookService.Namespace,
						Port:      &servicePort,
						Path:      ptr.To("/mutate-telemetry-kyma-project-io-v1alpha1-metricpipeline"),
					},
				},
				FailurePolicy:  &failurePolicy,
				MatchPolicy:    &matchPolicy,
				Name:           "mutating.v1alpha1.metricpipelines.telemetry.kyma-project.io",
				SideEffects:    &sideEffects,
				TimeoutSeconds: &timeout,
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: operations,
						Rule: admissionregistrationv1.Rule{
							APIGroups:   apiGroups,
							APIVersions: apiVersions,
							Scope:       &scope,
							Resources:   []string{"metricpipelines"},
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
						Path:      ptr.To("/mutate-telemetry-kyma-project-io-v1alpha1-tracepipeline"),
					},
				},
				FailurePolicy:  &failurePolicy,
				MatchPolicy:    &matchPolicy,
				Name:           "mutating.v1alpha1.tracepipelines.telemetry.kyma-project.io",
				SideEffects:    &sideEffects,
				TimeoutSeconds: &timeout,
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: operations,
						Rule: admissionregistrationv1.Rule{
							APIGroups:   apiGroups,
							APIVersions: apiVersions,
							Scope:       &scope,
							Resources:   []string{"tracepipelines"},
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
						Path:      ptr.To("/mutate-telemetry-kyma-project-io-v1alpha1-logpipeline"),
					},
				},
				FailurePolicy:  &failurePolicy,
				MatchPolicy:    &matchPolicy,
				Name:           "mutating.v1alpha1.logpipelines.telemetry.kyma-project.io",
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
		},
	}
)

func TestUpdatePipelineWithWebhookConfig(t *testing.T) {
	tests := []struct {
		name string
		crd  *apiextensionsv1.CustomResourceDefinition
	}{
		{
			name: "logpipeline",
			crd:  &logPipelinesCRD,
		},
		{
			name: "metricpipeline",
			crd:  &metricPipelinesCRD,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, clientgoscheme.AddToScheme(scheme))
			require.NoError(t, apiextensionsv1.AddToScheme(scheme))
			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&logPipelinesCRD, &metricPipelinesCRD, &validatingWebhookConfiguration, &mutatingWebhookConfiguration).Build()

			certDir := t.TempDir()
			defer func(path string) {
				deleteErr := os.RemoveAll(path)
				require.NoError(t, deleteErr)
			}(certDir)

			config := Config{
				rsaKeySize:            testRsaKeySize,
				CertDir:               certDir,
				ServiceName:           webhookService,
				CASecretName:          caBundleSecret,
				ValidatingWebhookName: validatingWebhookNamespacedName,
				MutatingWebhookName:   mutatingWebhookNamespacedName,
			}

			err := EnsureCertificate(t.Context(), client, config)
			require.NoError(t, err)

			serverCert, err := os.ReadFile(path.Join(certDir, "tls.crt"))
			require.NoError(t, err)

			var crd apiextensionsv1.CustomResourceDefinition
			require.NoError(t, client.Get(t.Context(), types.NamespacedName{Name: tt.crd.Name}, &crd))

			require.Equal(t, apiextensionsv1.WebhookConverter, crd.Spec.Conversion.Strategy)
			require.Equal(t, webhookService.Name, crd.Spec.Conversion.Webhook.ClientConfig.Service.Name)
			require.Equal(t, webhookService.Namespace, crd.Spec.Conversion.Webhook.ClientConfig.Service.Namespace)
			require.Equal(t, int32(443), *crd.Spec.Conversion.Webhook.ClientConfig.Service.Port)
			require.Equal(t, "/convert", *crd.Spec.Conversion.Webhook.ClientConfig.Service.Path)

			crdCABundle := crd.Spec.Conversion.Webhook.ClientConfig.CABundle
			require.NotEmpty(t, crdCABundle)

			var chainChecker certChainCheckerImpl

			certValid, err := chainChecker.checkRoot(t.Context(), serverCert, crdCABundle)
			require.NoError(t, err)
			require.True(t, certValid)
		})
	}
}

func TestUpdateWebhookConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, apiextensionsv1.AddToScheme(scheme))

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&logPipelinesCRD, &metricPipelinesCRD, &validatingWebhookConfiguration, &mutatingWebhookConfiguration).Build()

	certDir := t.TempDir()

	defer func(path string) {
		deleteErr := os.RemoveAll(path)
		require.NoError(t, deleteErr)
	}(certDir)

	config := Config{
		rsaKeySize:            testRsaKeySize,
		CertDir:               certDir,
		ServiceName:           webhookService,
		CASecretName:          caBundleSecret,
		ValidatingWebhookName: validatingWebhookNamespacedName,
		MutatingWebhookName:   mutatingWebhookNamespacedName,
	}

	err := EnsureCertificate(t.Context(), client, config)
	require.NoError(t, err)

	newServerCert, err := os.ReadFile(path.Join(certDir, "tls.crt"))
	require.NoError(t, err)

	var updatedValidatingWebhookConfiguration admissionregistrationv1.ValidatingWebhookConfiguration

	err = client.Get(t.Context(), config.ValidatingWebhookName, &updatedValidatingWebhookConfiguration)
	require.NoError(t, err)

	var chainChecker certChainCheckerImpl

	require.Len(t, updatedValidatingWebhookConfiguration.Webhooks, 1)

	certValid, err := chainChecker.checkRoot(t.Context(), newServerCert, updatedValidatingWebhookConfiguration.Webhooks[0].ClientConfig.CABundle)
	require.NoError(t, err)
	require.True(t, certValid)

	var updatedMutatingWebhookConfiguration admissionregistrationv1.MutatingWebhookConfiguration

	err = client.Get(t.Context(), config.MutatingWebhookName, &updatedMutatingWebhookConfiguration)
	require.NoError(t, err)

	mutatingCertValid, err := chainChecker.checkRoot(t.Context(), newServerCert, updatedMutatingWebhookConfiguration.Webhooks[0].ClientConfig.CABundle)
	require.NoError(t, err)
	require.True(t, mutatingCertValid)

	mutatingCertValid, err = chainChecker.checkRoot(t.Context(), newServerCert, updatedMutatingWebhookConfiguration.Webhooks[1].ClientConfig.CABundle)
	require.NoError(t, err)
	require.True(t, mutatingCertValid)

	mutatingCertValid, err = chainChecker.checkRoot(t.Context(), newServerCert, updatedMutatingWebhookConfiguration.Webhooks[2].ClientConfig.CABundle)
	require.NoError(t, err)
	require.True(t, mutatingCertValid)
}

func TestNewWebhookCertConfig(t *testing.T) {
	t.Run("should initialize all fields correctly", func(t *testing.T) {
		certDir := t.TempDir()

		config := NewWebhookCertConfig(
			ConfigOptions{
				CertDir:               certDir,
				ServiceName:           webhookService,
				CASecretName:          caBundleSecret,
				ValidatingWebhookName: validatingWebhookNamespacedName,
				MutatingWebhookName:   mutatingWebhookNamespacedName,
			},
		)

		require.Equal(t, rsaKeySize, config.rsaKeySize)
		require.Equal(t, certDir, config.CertDir)
		require.Equal(t, webhookService, config.ServiceName)
		require.Equal(t, caBundleSecret, config.CASecretName)
		require.Equal(t, validatingWebhookNamespacedName, config.ValidatingWebhookName)
		require.Equal(t, mutatingWebhookNamespacedName, config.MutatingWebhookName)
	})
}

func TestCreateSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, apiextensionsv1.AddToScheme(scheme))
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&logPipelinesCRD, &metricPipelinesCRD, &validatingWebhookConfiguration, &mutatingWebhookConfiguration).Build()

	certDir := t.TempDir()

	defer func(path string) {
		deleteErr := os.RemoveAll(path)
		require.NoError(t, deleteErr)
	}(certDir)

	config := Config{
		rsaKeySize:            testRsaKeySize,
		CertDir:               certDir,
		ServiceName:           webhookService,
		CASecretName:          caBundleSecret,
		ValidatingWebhookName: validatingWebhookNamespacedName,
		MutatingWebhookName:   mutatingWebhookNamespacedName,
	}

	err := EnsureCertificate(t.Context(), client, config)
	require.NoError(t, err)

	var secret corev1.Secret

	err = client.Get(t.Context(), config.CASecretName, &secret)
	require.NoError(t, err)

	require.Contains(t, secret.Data, "ca.crt")
	require.Contains(t, secret.Data, "ca.key")
}

func TestReuseExistingCertificate(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, apiextensionsv1.AddToScheme(scheme))
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&logPipelinesCRD, &metricPipelinesCRD, &validatingWebhookConfiguration, &mutatingWebhookConfiguration).Build()

	certDir := t.TempDir()

	defer func(path string) {
		deleteErr := os.RemoveAll(path)
		require.NoError(t, deleteErr)
	}(certDir)

	config := Config{
		rsaKeySize:            testRsaKeySize,
		CertDir:               certDir,
		ServiceName:           webhookService,
		CASecretName:          caBundleSecret,
		ValidatingWebhookName: validatingWebhookNamespacedName,
		MutatingWebhookName:   mutatingWebhookNamespacedName,
	}

	err := EnsureCertificate(t.Context(), client, config)
	require.NoError(t, err)

	var newValidatingWebhookConfiguration admissionregistrationv1.ValidatingWebhookConfiguration

	err = client.Get(t.Context(), config.ValidatingWebhookName, &newValidatingWebhookConfiguration)
	require.NoError(t, err)

	var newMutatingWebhookConfiguration admissionregistrationv1.MutatingWebhookConfiguration

	err = client.Get(t.Context(), config.MutatingWebhookName, &newMutatingWebhookConfiguration)
	require.NoError(t, err)

	err = EnsureCertificate(t.Context(), client, config)
	require.NoError(t, err)

	var updatedValidatingWebhookConfiguration admissionregistrationv1.ValidatingWebhookConfiguration

	err = client.Get(t.Context(), config.ValidatingWebhookName, &updatedValidatingWebhookConfiguration)
	require.NoError(t, err)

	require.Len(t, updatedValidatingWebhookConfiguration.Webhooks, 1)

	require.Equal(t, newValidatingWebhookConfiguration.Webhooks[0].ClientConfig.CABundle,
		updatedValidatingWebhookConfiguration.Webhooks[0].ClientConfig.CABundle)

	var updatedMutatingWebhookConfiguration admissionregistrationv1.MutatingWebhookConfiguration

	err = client.Get(t.Context(), config.MutatingWebhookName, &updatedMutatingWebhookConfiguration)
	require.NoError(t, err)

	require.Equal(t, newMutatingWebhookConfiguration.Webhooks[0].ClientConfig.CABundle,
		updatedMutatingWebhookConfiguration.Webhooks[0].ClientConfig.CABundle)
	require.Equal(t, newMutatingWebhookConfiguration.Webhooks[1].ClientConfig.CABundle,
		updatedMutatingWebhookConfiguration.Webhooks[1].ClientConfig.CABundle)
	require.Equal(t, newMutatingWebhookConfiguration.Webhooks[2].ClientConfig.CABundle,
		updatedMutatingWebhookConfiguration.Webhooks[2].ClientConfig.CABundle)
}
