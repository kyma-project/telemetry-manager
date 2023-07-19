package telemetry

import (
	"context"
	"fmt"
	"testing"
	"time"

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

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/metricpipeline"
	gatewayresources "github.com/kyma-project/telemetry-manager/internal/resources/otelcollector/gateway"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	testMetricPipelineReconcilerConfig = metricpipeline.Config{
		Gateway: gatewayresources.Config{
			BaseName:  "telemetry-metric-gateway",
			Namespace: "telemetry-system",
			Deployment: gatewayresources.DeploymentConfig{
				Image:             "otel/opentelemetry-collector-contrib:0.60.0",
				BaseCPULimit:      resource.MustParse("1"),
				BaseMemoryLimit:   resource.MustParse("1Gi"),
				BaseCPURequest:    resource.MustParse("150m"),
				BaseMemoryRequest: resource.MustParse("256Mi"),
				PriorityClassName: "telemetry-priority-class",
			},
			Service: gatewayresources.ServiceConfig{
				OTLPServiceName: "telemetry-otlp-metrics",
			},
		},
		OverridesConfigMapName: types.NamespacedName{Name: "override-config", Namespace: "telemetry-system"},
	}
)

var _ = Describe("Deploying a MetricPipeline", Ordered, func() {
	const (
		timeout  = time.Second * 100
		interval = time.Millisecond * 250
	)

	BeforeAll(func() {
		ctx := context.Background()
		data := map[string][]byte{
			"user":     []byte("secret-username"),
			"password": []byte("secret-password"),
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "basic-auth-credentials-metrics",
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
											Name:      "basic-auth-credentials-metrics",
											Namespace: "default",
											Key:       "user",
										},
									},
								},
								Password: telemetryv1alpha1.ValueType{
									ValueFrom: &telemetryv1alpha1.ValueFromSource{
										SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
											Name:      "basic-auth-credentials-metrics",
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

		Expect(k8sClient.Create(ctx, secret)).Should(Succeed())
		Expect(k8sClient.Create(ctx, metricPipeline)).Should(Succeed())

		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, metricPipeline)).Should(Succeed())
		})
	})

	It("creates OpenTelemetry Collector resources", func() {
		Eventually(func() error {
			var serviceAccount corev1.ServiceAccount
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "telemetry-metric-gateway",
				Namespace: "telemetry-system",
			}, &serviceAccount); err != nil {
				return err
			}
			return validateMetricsOwnerReferences(serviceAccount.OwnerReferences)
		}, timeout, interval).Should(BeNil())

		Eventually(func() error {
			var clusterRole rbacv1.ClusterRole
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "telemetry-metric-gateway",
				Namespace: "telemetry-system",
			}, &clusterRole); err != nil {
				return err
			}
			return validateMetricsOwnerReferences(clusterRole.OwnerReferences)
		}, timeout, interval).Should(BeNil())

		Eventually(func() error {
			var clusterRoleBinding rbacv1.ClusterRoleBinding
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "telemetry-metric-gateway",
				Namespace: "telemetry-system",
			}, &clusterRoleBinding); err != nil {
				return err
			}
			return validateMetricsOwnerReferences(clusterRoleBinding.OwnerReferences)
		}, timeout, interval).Should(BeNil())

		Eventually(func() error {
			var otelCollectorDeployment appsv1.Deployment
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "telemetry-metric-gateway",
				Namespace: "telemetry-system",
			}, &otelCollectorDeployment); err != nil {
				return err
			}
			if err := validateMetricsOwnerReferences(otelCollectorDeployment.OwnerReferences); err != nil {
				return err
			}
			if err := validateMetricsEnvironment(otelCollectorDeployment); err != nil {
				return err
			}
			return validatePodAnnotations(otelCollectorDeployment)
		}, timeout, interval).Should(BeNil())

		Eventually(func() error {
			var otelCollectorService corev1.Service
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "telemetry-otlp-metrics",
				Namespace: "telemetry-system",
			}, &otelCollectorService); err != nil {
				return err
			}
			return validateMetricsOwnerReferences(otelCollectorService.OwnerReferences)
		}, timeout, interval).Should(BeNil())

		Eventually(func() map[string]string {
			var otelCollectorConfigMap corev1.ConfigMap
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "telemetry-metric-gateway",
				Namespace: "telemetry-system",
			}, &otelCollectorConfigMap); err != nil {
				return nil
			}
			if err := validateMetricsOwnerReferences(otelCollectorConfigMap.OwnerReferences); err != nil {
				return nil
			}
			return otelCollectorConfigMap.Data
		}, timeout, interval).Should(HaveKey("relay.conf"))

		Eventually(func() error {
			var otelCollectorSecret corev1.Secret
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "telemetry-metric-gateway",
				Namespace: "telemetry-system",
			}, &otelCollectorSecret); err != nil {
				return err
			}
			return validateMetricsOwnerReferences(otelCollectorSecret.OwnerReferences)
		}, timeout, interval).Should(BeNil())

		Eventually(func() error {
			var otelCollectorSecret corev1.Secret
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "telemetry-metric-gateway",
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
				Name:      "telemetry-metric-gateway",
				Namespace: "telemetry-system",
			}, &oteCollectorDep)).To(Succeed())
			priorityClassName := oteCollectorDep.Spec.Template.Spec.PriorityClassName
			g.Expect(priorityClassName).To(Equal("telemetry-priority-class"))
		}, timeout, interval).Should(Succeed())
	})

	It("updates Metric Collector Secret when referenced secret changes", func() {
		Eventually(func() error {
			newData := map[string][]byte{
				"user":     []byte("new-secret-username"),
				"password": []byte("new-secret-password"),
			}
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-auth-credentials-metrics",
					Namespace: "default",
				},
				Data: newData,
			}

			if err := k8sClient.Update(ctx, secret); err != nil {
				return err
			}

			var otelCollectorSecret corev1.Secret
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "telemetry-metric-gateway",
				Namespace: "telemetry-system",
			}, &otelCollectorSecret); err != nil {
				return err
			}

			return validateSecret(otelCollectorSecret, "new-secret-username", "new-secret-password")
		}, timeout, interval).Should(BeNil())
	})

})

func TestMetricPipeline_MapSecret(t *testing.T) {
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
						Name:      "basic-auth-credentials-metrics",
						Namespace: "default",
						Key:       "password",
					},
				},
			},
			secret: corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-auth-credentials-metrics",
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
					Name:      "basic-auth-credentials-metrics",
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

func validateMetricsEnvironment(deployment appsv1.Deployment) error {
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

func validateMetricsOwnerReferences(ownerReferences []metav1.OwnerReference) error {
	if len(ownerReferences) != 1 {
		return fmt.Errorf("unexpected number of owner references: %d", len(ownerReferences))
	}
	ownerReference := ownerReferences[0]

	if ownerReference.Kind != "MetricPipeline" {
		return fmt.Errorf("unexpected owner reference type: %s", ownerReference.Kind)
	}

	if ownerReference.Name != "dummy" {
		return fmt.Errorf("unexpected owner reference name: %s", ownerReference.Kind)
	}

	return nil
}
