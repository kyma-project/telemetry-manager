package telemetry

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
)

var (
	testTracePipelineReconcilerConfig = tracepipeline.Config{
		Gateway: otelcollector.GatewayConfig{
			Config: otelcollector.Config{
				Namespace: "telemetry-system",
				BaseName:  "telemetry-trace-collector",
			},
			Deployment: otelcollector.DeploymentConfig{
				Image:             "otel/opentelemetry-collector-contrib:0.73.0",
				BaseCPULimit:      resource.MustParse("1"),
				BaseMemoryLimit:   resource.MustParse("1Gi"),
				BaseCPURequest:    resource.MustParse("150m"),
				BaseMemoryRequest: resource.MustParse("256Mi"),
				PriorityClassName: "telemetry-priority-class",
			},
			OTLPServiceName: "telemetry-otlp-traces",
		},
		OverridesConfigMapName: types.NamespacedName{Name: "override-config", Namespace: "telemetry-system"},
		MaxPipelines:           0,
	}
)

var _ = Describe("Deploying a TracePipeline", Ordered, func() {
	const (
		timeout                = time.Second * 100
		interval               = time.Millisecond * 250
		customHeaderName       = "Token"
		customHeaderPrefix     = "Api-Token"
		customHeaderPlainValue = "foo_token"

		customHeaderNameForSecret   = "Authorization"
		customHeaderPrefixForSecret = "Bearer"
		customHeaderSecretKey       = "headerKey"
		customHeaderSecretData      = "bar_token"
	)

	BeforeAll(func() {
		ctx := context.Background()
		data := map[string][]byte{
			"user":     []byte("secret-username"),
			"password": []byte("secret-password"),
		}

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "basic-auth-credentials-tracing",
				Namespace: "default",
			},
			Data: data,
		}

		customHeaderSecretData := map[string][]byte{
			customHeaderSecretKey: []byte(customHeaderSecretData),
		}

		customHeaderSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "custom-auth-header-tracing",
				Namespace: "default",
			},
			Data: customHeaderSecretData,
		}

		tracePipeline := &telemetryv1alpha1.TracePipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: "dummy",
			},
			Spec: telemetryv1alpha1.TracePipelineSpec{
				Output: telemetryv1alpha1.TracePipelineOutput{
					Otlp: &telemetryv1alpha1.OtlpOutput{
						Endpoint: telemetryv1alpha1.ValueType{Value: "http://localhost"},
						Authentication: &telemetryv1alpha1.AuthenticationOptions{
							Basic: &telemetryv1alpha1.BasicAuthOptions{
								User: telemetryv1alpha1.ValueType{
									ValueFrom: &telemetryv1alpha1.ValueFromSource{
										SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
											Name:      "basic-auth-credentials-tracing",
											Namespace: "default",
											Key:       "user",
										},
									},
								},
								Password: telemetryv1alpha1.ValueType{
									ValueFrom: &telemetryv1alpha1.ValueFromSource{
										SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
											Name:      "basic-auth-credentials-tracing",
											Namespace: "default",
											Key:       "password",
										},
									},
								},
							},
						},
						Headers: []telemetryv1alpha1.Header{
							{
								Name:   customHeaderName,
								Prefix: customHeaderPrefix,
								ValueType: telemetryv1alpha1.ValueType{
									Value: customHeaderPlainValue,
								},
							},
							{
								Name:   customHeaderNameForSecret,
								Prefix: customHeaderPrefixForSecret,
								ValueType: telemetryv1alpha1.ValueType{
									ValueFrom: &telemetryv1alpha1.ValueFromSource{
										SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
											Key:       customHeaderSecretKey,
											Name:      "custom-auth-header-tracing",
											Namespace: "default",
										},
									},
								},
							},
						},
					},
				},
			},
		}

		Expect(k8sClient.Create(ctx, secret)).Should(Succeed())
		Expect(k8sClient.Create(ctx, customHeaderSecret)).Should(Succeed())
		Expect(k8sClient.Create(ctx, tracePipeline)).Should(Succeed())

		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, customHeaderSecret)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, tracePipeline)).Should(Succeed())
		})
	})

	It("creates OpenTelemetry Collector resources", func() {
		Eventually(func() error {
			var serviceAccount corev1.ServiceAccount
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "telemetry-trace-collector",
				Namespace: "telemetry-system",
			}, &serviceAccount); err != nil {
				return err
			}
			return validateTracingOwnerReferences(serviceAccount.OwnerReferences)
		}, timeout, interval).Should(BeNil())

		Eventually(func() error {
			var clusterRole rbacv1.ClusterRole
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "telemetry-trace-collector",
				Namespace: "telemetry-system",
			}, &clusterRole); err != nil {
				return err
			}
			return validateTracingOwnerReferences(clusterRole.OwnerReferences)
		}, timeout, interval).Should(BeNil())

		Eventually(func() error {
			var clusterRoleBinding rbacv1.ClusterRoleBinding
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "telemetry-trace-collector",
				Namespace: "telemetry-system",
			}, &clusterRoleBinding); err != nil {
				return err
			}
			return validateTracingOwnerReferences(clusterRoleBinding.OwnerReferences)
		}, timeout, interval).Should(BeNil())

		Eventually(func() error {
			var otelCollectorDeployment appsv1.Deployment
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "telemetry-trace-collector",
				Namespace: "telemetry-system",
			}, &otelCollectorDeployment); err != nil {
				return err
			}
			if err := validateTracingOwnerReferences(otelCollectorDeployment.OwnerReferences); err != nil {
				return err
			}
			if err := validateTracingEnvironment(otelCollectorDeployment); err != nil {
				return err
			}
			return validatePodMetadata(otelCollectorDeployment)
		}, timeout, interval).Should(BeNil())

		Eventually(func() error {
			var otelCollectorService corev1.Service
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "telemetry-otlp-traces",
				Namespace: "telemetry-system",
			}, &otelCollectorService); err != nil {
				return err
			}
			return validateTracingOwnerReferences(otelCollectorService.OwnerReferences)
		}, timeout, interval).Should(BeNil())

		Eventually(func() map[string]string {
			var otelCollectorConfigMap corev1.ConfigMap
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "telemetry-trace-collector",
				Namespace: "telemetry-system",
			}, &otelCollectorConfigMap); err != nil {
				return nil
			}
			if err := validateTracingOwnerReferences(otelCollectorConfigMap.OwnerReferences); err != nil {
				return nil
			}
			return otelCollectorConfigMap.Data
		}, timeout, interval).Should(HaveKey("relay.conf"))

		Eventually(func() error {
			var otelCollectorSecret corev1.Secret
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "telemetry-trace-collector",
				Namespace: "telemetry-system",
			}, &otelCollectorSecret); err != nil {
				return err
			}
			return validateTracingOwnerReferences(otelCollectorSecret.OwnerReferences)
		}, timeout, interval).Should(BeNil())

		Eventually(func() error {
			var otelCollectorSecret corev1.Secret
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "telemetry-trace-collector",
				Namespace: "telemetry-system",
			}, &otelCollectorSecret); err != nil {
				return err
			}

			return validateSecret(otelCollectorSecret, "secret-username", "secret-password")
		}, timeout, interval).Should(BeNil())

	})

	It("Should have the correct priority class", func() {
		Eventually(func(g Gomega) {
			var oteCollectorDep appsv1.Deployment
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "telemetry-trace-collector",
				Namespace: "telemetry-system",
			}, &oteCollectorDep)).To(Succeed())
			priorityClassName := oteCollectorDep.Spec.Template.Spec.PriorityClassName
			g.Expect(priorityClassName).To(Equal("telemetry-priority-class"))
		}, timeout, interval).Should(Succeed())
	})

	It("Should update trace gateway secret when referenced secret changes", func() {
		Eventually(func() error {
			newData := map[string][]byte{
				"user":     []byte("new-secret-username"),
				"password": []byte("new-secret-password"),
			}
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-auth-credentials-tracing",
					Namespace: "default",
				},
				Data: newData,
			}

			if err := k8sClient.Update(ctx, secret); err != nil {
				return err
			}

			var otelCollectorSecret corev1.Secret
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "telemetry-trace-collector",
				Namespace: "telemetry-system",
			}, &otelCollectorSecret); err != nil {
				return err
			}

			return validateSecret(otelCollectorSecret, "new-secret-username", "new-secret-password")
		}, timeout, interval).Should(BeNil())
	})

	It("Should have a secret with custom header value and prefix", func() {
		Eventually(func() error {
			var otelCollectorSecret corev1.Secret
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "telemetry-trace-collector",
				Namespace: "telemetry-system",
			}, &otelCollectorSecret); err != nil {
				return err
			}

			return validateSecretDataWithKey(otelCollectorSecret, "HEADER_DUMMY_TOKEN", fmt.Sprintf("%s %s", customHeaderPrefix, customHeaderPlainValue))
		}, timeout, interval).Should(BeNil())
	})

	It("Should have a secret with custom header value and prefix from secret value", func() {
		Eventually(func() error {
			var otelCollectorSecret corev1.Secret
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "telemetry-trace-collector",
				Namespace: "telemetry-system",
			}, &otelCollectorSecret); err != nil {
				return err
			}

			return validateSecretDataWithKey(otelCollectorSecret, "HEADER_DUMMY_AUTHORIZATION", fmt.Sprintf("%s %s", customHeaderPrefixForSecret, customHeaderSecretData))
		}, timeout, interval).Should(BeNil())
	})

})

func validateTracingEnvironment(deployment appsv1.Deployment) error {
	container := deployment.Spec.Template.Spec.Containers[0]
	env := container.EnvFrom[0]

	if env.SecretRef.LocalObjectReference.Name != "telemetry-trace-collector" {
		return fmt.Errorf("unexpected secret name: %s", env.SecretRef.LocalObjectReference.Name)
	}

	if !*env.SecretRef.Optional {
		return fmt.Errorf("secret reference for environment must be optional")
	}

	return nil
}

func validateTracingOwnerReferences(ownerReferences []metav1.OwnerReference) error {
	if len(ownerReferences) != 1 {
		return fmt.Errorf("unexpected number of owner references: %d", len(ownerReferences))
	}
	ownerReference := ownerReferences[0]

	if ownerReference.Kind != "TracePipeline" {
		return fmt.Errorf("unexpected owner reference type: %s", ownerReference.Kind)
	}

	if ownerReference.Name != "dummy" {
		return fmt.Errorf("unexpected owner reference name: %s", ownerReference.Kind)
	}

	return nil
}
