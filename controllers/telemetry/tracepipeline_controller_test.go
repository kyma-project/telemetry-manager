package telemetry

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	testTracePipelineConfig = tracepipeline.Config{
		BaseName:          "telemetry-trace-collector",
		Namespace:         "kyma-system",
		OverrideConfigMap: types.NamespacedName{Name: "override-config", Namespace: "kyma-system"},
		Deployment: tracepipeline.DeploymentConfig{
			Image:         "otel/opentelemetry-collector-contrib:0.60.0",
			CPULimit:      resource.MustParse("1"),
			MemoryLimit:   resource.MustParse("1Gi"),
			CPURequest:    resource.MustParse("150m"),
			MemoryRequest: resource.MustParse("256Mi"),
		},
		Service: tracepipeline.ServiceConfig{
			OTLPServiceName: "telemetry-otlp-traces",
		},
	}
)

var _ = Describe("Deploying a TracePipeline", func() {
	const (
		timeout  = time.Second * 100
		interval = time.Millisecond * 250
	)

	When("creating TracePipeline", func() {
		ctx := context.Background()
		kymaSystemNamespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kyma-system",
			},
		}
		data := map[string][]byte{
			"user":     []byte("secret-username"),
			"password": []byte("secret-password"),
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "basic-auth-credentials",
				Namespace: "default",
			},
			Data: data,
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
			Expect(k8sClient.Create(ctx, tracePipeline)).Should(Succeed())

			Eventually(func() error {
				var serviceAccount corev1.ServiceAccount
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "telemetry-trace-collector",
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
					Name:      "telemetry-trace-collector",
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
					Name:      "telemetry-trace-collector",
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
					Name:      "telemetry-trace-collector",
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
				var otelCollectorService corev1.Service
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "telemetry-otlp-traces",
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
				var otelCollectorConfigMap corev1.ConfigMap
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "telemetry-trace-collector",
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
				var otelCollectorSecret corev1.Secret
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "telemetry-trace-collector",
					Namespace: "kyma-system",
				}, &otelCollectorSecret); err != nil {
					return err
				}
				if err := validateOwnerReferences(otelCollectorSecret.OwnerReferences); err != nil {
					return err
				}
				return nil
			}, timeout, interval).Should(BeNil())

			Expect(k8sClient.Delete(ctx, tracePipeline)).Should(Succeed())
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

	if env.SecretRef.LocalObjectReference.Name != "telemetry-trace-collector" {
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

	if ownerReference.Kind != "TracePipeline" {
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
	var config tracepipeline.OTELCollectorConfig
	if err := yaml.Unmarshal([]byte(configData), &config); err != nil {
		return err
	}

	if !config.Exporters.OTLP.TLS.Insecure {
		return fmt.Errorf("Insecure flag not set")
	}

	return nil
}

func TestMapSecret(t *testing.T) {
	tests := []struct {
		password         telemetryv1alpha1.ValueType
		secret           corev1.Secret
		summary          string
		expectedRequests []reconcile.Request
	}{
		{
			summary: "map secret referenced by pipeline",
			password: telemetryv1alpha1.ValueType{
				ValueFrom: &telemetryv1alpha1.ValueFromSource{
					SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
						Name:      "basic-auth-credentials",
						Namespace: "default",
						Key:       "password",
					},
				},
			},
			secret: corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-auth-credentials",
					Namespace: "default",
				},
			},
			expectedRequests: []reconcile.Request{{NamespacedName: types.NamespacedName{Name: "dummy"}}},
		},
		{
			summary: "map unused secret",
			password: telemetryv1alpha1.ValueType{
				Value: "qwerty",
			},
			secret: corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-auth-credentials",
					Namespace: "default",
				},
			},
			expectedRequests: []reconcile.Request{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.summary, func(t *testing.T) {
			tracePipeline := &telemetryv1alpha1.TracePipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "dummy",
				},
				Spec: telemetryv1alpha1.TracePipelineSpec{
					Output: telemetryv1alpha1.TracePipelineOutput{
						Otlp: &telemetryv1alpha1.OtlpOutput{
							Endpoint: telemetryv1alpha1.ValueType{Value: "localhost"},
							Authentication: &telemetryv1alpha1.AuthenticationOptions{
								Basic: &telemetryv1alpha1.BasicAuthOptions{
									User:     telemetryv1alpha1.ValueType{Value: "admin"},
									Password: tc.password,
								},
							},
						},
					},
				},
			}

			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = telemetryv1alpha1.AddToScheme(scheme)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tracePipeline).Build()
			sut := TracePipelineReconciler{Client: fakeClient}

			actualRequests := sut.mapSecret(&tc.secret)

			require.ElementsMatch(t, tc.expectedRequests, actualRequests)
		})
	}
}
