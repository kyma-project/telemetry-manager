package telemetry

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline"
	"github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"
)

var (
	testLogPipelineConfig = logpipeline.Config{
		DaemonSet:             types.NamespacedName{Name: "test-telemetry-fluent-bit", Namespace: "default"},
		ParsersConfigMap:      types.NamespacedName{Name: "test-telemetry-fluent-bit-parsers", Namespace: "default"},
		LuaConfigMap:          types.NamespacedName{Name: "test-telemetry-fluent-bit-luascripts", Namespace: "default"},
		SectionsConfigMap:     types.NamespacedName{Name: "test-telemetry-fluent-bit-sections", Namespace: "default"},
		FilesConfigMap:        types.NamespacedName{Name: "test-telemetry-fluent-bit-files", Namespace: "default"},
		EnvSecret:             types.NamespacedName{Name: "test-telemetry-fluent-bit-env", Namespace: "default"},
		OutputTLSConfigSecret: types.NamespacedName{Name: "test-telemetry-fluent-bit-output-tls-config", Namespace: "default"},
		OverrideConfigMap:     types.NamespacedName{Name: "override-config", Namespace: "default"},
		DaemonSetConfig: fluentbit.DaemonSetConfig{
			FluentBitImage:              "my-fluent-bit-image",
			FluentBitConfigPrepperImage: "my-fluent-bit-config-image",
			ExporterImage:               "my-exporter-image",
			PriorityClassName:           "telemetry-priority-class-high",
			CPULimit:                    resource.MustParse("1"),
			MemoryLimit:                 resource.MustParse("500Mi"),
			CPURequest:                  resource.MustParse(".1"),
			MemoryRequest:               resource.MustParse("100Mi"),
		},
		PipelineDefaults: builder.PipelineDefaults{
			InputTag:          "kube",
			MemoryBufferLimit: "10M",
			StorageType:       "filesystem",
			FsBufferLimit:     "1G",
		},
	}
)

var _ = Describe("LogPipeline controller", Ordered, func() {
	const (
		LogPipelineName       = "log-pipeline"
		FluentBitFilterConfig = "Name   grep\nRegex   $kubernetes['labels']['app'] my-deployment"
		FluentBitOutputConfig = "Name   stdout\n"
		timeout               = time.Second * 10
		interval              = time.Millisecond * 250
	)

	file := telemetryv1alpha1.FileMount{
		Name:    "myFile",
		Content: "file-content",
	}

	secretKeyRef := telemetryv1alpha1.SecretKeyRef{
		Name:      "my-secret",
		Namespace: testLogPipelineConfig.DaemonSet.Namespace,
		Key:       "key",
	}

	variableRefs := telemetryv1alpha1.VariableRef{
		Name:      "myKey",
		ValueFrom: telemetryv1alpha1.ValueFromSource{SecretKeyRef: &secretKeyRef},
	}

	filter := telemetryv1alpha1.Filter{
		Custom: FluentBitFilterConfig,
	}

	var logPipeline = &telemetryv1alpha1.LogPipeline{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "telemetry.kyma-project.io/v1alpha1",
			Kind:       "LogPipeline",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: LogPipelineName,
		},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.Input{Application: telemetryv1alpha1.ApplicationInput{
				Namespaces: telemetryv1alpha1.InputNamespaces{
					System: true}}},
			Filters:   []telemetryv1alpha1.Filter{filter},
			Output:    telemetryv1alpha1.Output{Custom: FluentBitOutputConfig},
			Files:     []telemetryv1alpha1.FileMount{file},
			Variables: []telemetryv1alpha1.VariableRef{variableRefs},
		},
	}

	Context("When creating a LogPipeline", Ordered, func() {
		BeforeAll(func() {
			ctx := context.Background()
			secret := &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: testLogPipelineConfig.DaemonSet.Namespace,
				},
				StringData: map[string]string{
					"key": "value",
				},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())
			Expect(k8sClient.Create(ctx, logPipeline)).Should(Succeed())

		})

		It("Should have created a fluent-bit daemon set", func() {
			Eventually(func() error {
				var fluentBitDaemonSet appsv1.DaemonSet
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testLogPipelineConfig.DaemonSet.Name,
					Namespace: testLogPipelineConfig.DaemonSet.Namespace,
				}, &fluentBitDaemonSet)
				return err
			}, timeout, interval).Should(BeNil())
		})

		It("Should have the correct priority class", func() {
			Eventually(func(g Gomega) {
				var fluentBitDaemonSet appsv1.DaemonSet
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      testLogPipelineConfig.DaemonSet.Name,
					Namespace: testLogPipelineConfig.DaemonSet.Namespace,
				}, &fluentBitDaemonSet)).To(Succeed())
				priorityClassName := fluentBitDaemonSet.Spec.Template.Spec.PriorityClassName
				g.Expect(priorityClassName).To(Equal("telemetry-priority-class-high"))
			}, timeout, interval).Should(Succeed())
		})

		It("Should have the expected owner references", func() {
			Eventually(func() error {
				var fluentBitDaemonSet appsv1.DaemonSet
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testLogPipelineConfig.DaemonSet.Name,
					Namespace: testLogPipelineConfig.DaemonSet.Namespace,
				}, &fluentBitDaemonSet); err != nil {
					return err
				}
				return validateLoggingOwnerReferences(fluentBitDaemonSet.OwnerReferences)
			}, timeout, interval).Should(BeNil())
		})
	})
})

func validateLoggingOwnerReferences(ownerReferences []metav1.OwnerReference) error {
	if len(ownerReferences) != 1 {
		return fmt.Errorf("unexpected number of owner references: %d", len(ownerReferences))
	}
	ownerReference := ownerReferences[0]
	if ownerReference.Kind != "LogPipeline" {
		return fmt.Errorf("unexpected owner reference type: %s", ownerReference.Kind)
	}
	if ownerReference.Name != "log-pipeline" {
		return fmt.Errorf("unexpected owner reference name: %s", ownerReference.Kind)
	}

	return nil
}
