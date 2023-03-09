package telemetry

import (
	"context"
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/collector"
	"time"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Deploying a MetricPipeline", func() {
	const (
		timeout  = time.Second * 100
		interval = time.Millisecond * 250
	)

	When("creating MetricPipeline", func() {
		ctx := context.Background()
		kymaSystemNamespace := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kyma-system",
			},
		}
		data := map[string][]byte{
			"user":     []byte("secret-username"),
			"password": []byte("secret-password"),
		}
		secret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "basic-auth-credentials",
				Namespace: "default",
			},
			Data: data,
		}
		metricPipeline := &telemetryv1alpha1.MetricPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: "dummy",
			},
			Spec: telemetryv1alpha1.MetricPipelineSpec{
				Output: telemetryv1alpha1.MetricPipelineOutput{
					Otlp: &telemetryv1alpha1.OtlpOutput{
						Endpoint: telemetryv1alpha1.ValueType{Value: "http://localhost"},
						Authentication: &telemetryv1alpha1.AuthenticationOptions{
							Basic: &telemetryv1alpha1.BasicAuthOptions{
								User: telemetryv1alpha1.ValueType{
									ValueFrom: &telemetryv1alpha1.ValueFromSource{
										SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
											Name:      "basic-auth-credentials",
											Namespace: "default",
											Key:       "user",
										},
									},
								},
								Password: telemetryv1alpha1.ValueType{
									ValueFrom: &telemetryv1alpha1.ValueFromSource{
										SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
											Name:      "basic-auth-credentials",
											Namespace: "default",
											Key:       "password",
										},
									},
								},
							},
						},
					},
				},
			},
		}

		It("creates OpenTelemetry Collector resources", func() {
			Expect(k8sClient.Create(ctx, kymaSystemNamespace)).Should(Succeed())
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())
			Expect(k8sClient.Create(ctx, metricPipeline)).Should(Succeed())

			Eventually(func() error {
				var serviceAccount v1.ServiceAccount
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "telemetry-metric-gateway",
					Namespace: "kyma-system",
				}, &serviceAccount); err != nil {
					return err
				}
				if err := validateOwnerReferences(serviceAccount.OwnerReferences); err != nil {
					return err
				}
				return nil
			}, timeout, interval).Should(BeNil())

			Eventually(func() error {
				var clusterRole rbacv1.ClusterRole
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "telemetry-metric-gateway",
					Namespace: "kyma-system",
				}, &clusterRole); err != nil {
					return err
				}
				if err := validateOwnerReferences(clusterRole.OwnerReferences); err != nil {
					return err
				}
				return nil
			}, timeout, interval).Should(BeNil())

			Eventually(func() error {
				var clusterRoleBinding rbacv1.ClusterRoleBinding
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "telemetry-metric-gateway",
					Namespace: "kyma-system",
				}, &clusterRoleBinding); err != nil {
					return err
				}
				if err := validateOwnerReferences(clusterRoleBinding.OwnerReferences); err != nil {
					return err
				}
				return nil
			}, timeout, interval).Should(BeNil())

			Eventually(func() error {
				var otelCollectorDeployment appsv1.Deployment
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "telemetry-metric-gateway",
					Namespace: "kyma-system",
				}, &otelCollectorDeployment); err != nil {
					return err
				}
				if err := validateOwnerReferences(otelCollectorDeployment.OwnerReferences); err != nil {
					return err
				}
				if err := validateEnvironment(otelCollectorDeployment); err != nil {
					return err
				}
				if err := validatePodAnnotations(otelCollectorDeployment); err != nil {
					return err
				}
				return nil
			}, timeout, interval).Should(BeNil())

			Eventually(func() error {
				var otelCollectorService v1.Service
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "telemetry-otlp-metrics",
					Namespace: "kyma-system",
				}, &otelCollectorService); err != nil {
					return err
				}
				if err := validateOwnerReferences(otelCollectorService.OwnerReferences); err != nil {
					return err
				}
				return nil
			}, timeout, interval).Should(BeNil())

			Eventually(func() error {
				var otelCollectorConfigMap v1.ConfigMap
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "telemetry-metric-gateway",
					Namespace: "kyma-system",
				}, &otelCollectorConfigMap); err != nil {
					return err
				}
				if err := validateOwnerReferences(otelCollectorConfigMap.OwnerReferences); err != nil {
					return err
				}
				if err := validateCollectorConfig(otelCollectorConfigMap.Data["relay.conf"]); err != nil {
					return err
				}
				return nil
			}, timeout, interval).Should(BeNil())

			Eventually(func() error {
				var otelCollectorSecret v1.Secret
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "telemetry-metric-gateway",
					Namespace: "kyma-system",
				}, &otelCollectorSecret); err != nil {
					return err
				}
				if err := validateOwnerReferences(otelCollectorSecret.OwnerReferences); err != nil {
					return err
				}
				return nil
			}, timeout, interval).Should(BeNil())

			Expect(k8sClient.Delete(ctx, metricPipeline)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, kymaSystemNamespace)).Should(Succeed())
		})
	})
})

func validatePodAnnotations(deployment appsv1.Deployment) error {
	if value, found := deployment.Spec.Template.ObjectMeta.Annotations["sidecar.istio.io/inject"]; !found || value != "false" {
		return fmt.Errorf("istio sidecar injection for otel collector not disabled")
	}

	if value, found := deployment.Spec.Template.ObjectMeta.Annotations["checksum/config"]; !found || value == "" {
		return fmt.Errorf("configuration hash not found in pod annotations")
	}

	return nil
}

func validateEnvironment(deployment appsv1.Deployment) error {
	container := deployment.Spec.Template.Spec.Containers[0]
	env := container.EnvFrom[0]

	if env.SecretRef.LocalObjectReference.Name != "telemetry-metric-gateway" {
		return fmt.Errorf("unexpected secret name: %s", env.SecretRef.LocalObjectReference.Name)
	}

	if !*env.SecretRef.Optional {
		return fmt.Errorf("secret reference for environment must be optional")
	}

	return nil
}

func validateOwnerReferences(ownerRefernces []metav1.OwnerReference) error {
	if len(ownerRefernces) != 1 {
		return fmt.Errorf("unexpected number of owner references: %d", len(ownerRefernces))
	}
	ownerReference := ownerRefernces[0]

	if ownerReference.Kind != "MetricPipeline" {
		return fmt.Errorf("unexpected owner reference type: %s", ownerReference.Kind)
	}

	if ownerReference.Name != "dummy" {
		return fmt.Errorf("unexpected owner reference name: %s", ownerReference.Kind)
	}

	if !*ownerReference.BlockOwnerDeletion {
		return fmt.Errorf("owner reference does not block owner deletion")
	}
	return nil
}

func validateCollectorConfig(configData string) error {
	var config collector.OTELCollectorConfig
	if err := yaml.Unmarshal([]byte(configData), &config); err != nil {
		return err
	}

	if !config.Exporters.OTLP.TLS.Insecure {
		return fmt.Errorf("Insecure flag not set")
	}

	return nil
}
