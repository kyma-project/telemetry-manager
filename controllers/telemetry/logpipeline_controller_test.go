package telemetry

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/common/expfmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	logpipelinereconciler "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline"
	logpipelineresources "github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	testLogPipelineConfig = logpipelinereconciler.Config{
		DaemonSet:         types.NamespacedName{Name: "test-telemetry-fluent-bit", Namespace: "default"},
		SectionsConfigMap: types.NamespacedName{Name: "test-telemetry-fluent-bit-sections", Namespace: "default"},
		FilesConfigMap:    types.NamespacedName{Name: "test-telemetry-fluent-bit-files", Namespace: "default"},
		EnvSecret:         types.NamespacedName{Name: "test-telemetry-fluent-bit-env", Namespace: "default"},
		OverrideConfigMap: types.NamespacedName{Name: "override-config", Namespace: "default"},
		DaemonSetConfig: logpipelineresources.DaemonSetConfig{
			FluentBitImage:              "my-fluent-bit-image",
			FluentBitConfigPrepperImage: "my-fluent-bit-config-image",
			ExporterImage:               "my-exporter-image",
			PriorityClassName:           "my-priority-class",
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
	var expectedFluentBitConfig = `[FILTER]
    name                  rewrite_tag
    match                 kube.*
    emitter_mem_buf_limit 10M
    emitter_name          log-pipeline-stdout
    emitter_storage.type  filesystem
    rule                  $log "^.*$" log-pipeline.$TAG true

[FILTER]
    name   record_modifier
    match  log-pipeline.*
    record cluster_identifier ${KUBERNETES_SERVICE_HOST}

[FILTER]
    name  grep
    match log-pipeline.*
    regex $kubernetes['labels']['app'] my-deployment

[FILTER]
    name         nest
    match        log-pipeline.*
    add_prefix   __kyma__
    nested_under kubernetes
    operation    lift

[FILTER]
    name       record_modifier
    match      log-pipeline.*
    remove_key __kyma__annotations

[FILTER]
    name          nest
    match         log-pipeline.*
    nest_under    kubernetes
    operation     nest
    remove_prefix __kyma__
    wildcard      __kyma__*

[OUTPUT]
    name                     stdout
    match                    log-pipeline.*
    alias                    log-pipeline-stdout
    retry_limit              no_limits
    storage.total_limit_size 1G`

	var expectedSecret = make(map[string][]byte)
	expectedSecret["myKey"] = []byte("value")
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
	Context("On startup", Ordered, func() {
		It("Should not have any Logpipelines", func() {
			ctx := context.Background()
			var logPipelineList telemetryv1alpha1.LogPipelineList
			err := k8sClient.List(ctx, &logPipelineList)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(logPipelineList.Items).Should(BeEmpty())
		})
		It("Should not have any fluent-bit daemon set", func() {
			var fluentBitDaemonSetList appsv1.DaemonSetList
			err := k8sClient.List(ctx, &fluentBitDaemonSetList)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(fluentBitDaemonSetList.Items).Should(BeEmpty())
		})
	})
	Context("When creating a log pipeline", Ordered, func() {
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
		It("Should verify metrics from the telemetry operator are exported", func() {
			Eventually(func() bool {
				resp, err := http.Get("http://localhost:8080/metrics")
				if err != nil {
					return false
				}
				defer resp.Body.Close()
				scanner := bufio.NewScanner(resp.Body)
				for scanner.Scan() {
					line := scanner.Text()
					if strings.Contains(line, "telemetry_all_logpipelines") || strings.Contains(line, "telemetry_unsupported_logpipelines") {
						return true
					}
				}
				return false
			}, timeout, interval).Should(Equal(true))
		})
		It("Should have the telemetry_all_logpipelines metric set to 1", func() {
			// All log pipeline gauge should be updated
			Eventually(func() float64 {
				resp, err := http.Get("http://localhost:8080/metrics")
				if err != nil {
					return 0
				}
				var parser expfmt.TextParser
				mf, err := parser.TextToMetricFamilies(resp.Body)
				if err != nil {
					return 0
				}

				return *mf["telemetry_all_logpipelines"].Metric[0].Gauge.Value
			}, timeout, interval).Should(Equal(1.0))
		})
		It("Should have the telemetry_unsupported_logpipelines metric set to 1", func() {
			Eventually(func() float64 {
				resp, err := http.Get("http://localhost:8080/metrics")
				if err != nil {
					return 0
				}
				var parser expfmt.TextParser
				mf, err := parser.TextToMetricFamilies(resp.Body)
				if err != nil {
					return 0
				}

				return *mf["telemetry_unsupported_logpipelines"].Metric[0].Gauge.Value
			}, timeout, interval).Should(Equal(1.0))
		})
		It("Should have fluent bit config section copied to the Fluent Bit configmap", func() {
			// Fluent Bit config section should be copied to ConfigMap
			Eventually(func() string {
				cmFileName := LogPipelineName + ".conf"
				configMapLookupKey := types.NamespacedName{
					Name:      testLogPipelineConfig.SectionsConfigMap.Name,
					Namespace: testLogPipelineConfig.SectionsConfigMap.Namespace,
				}
				var fluentBitCm corev1.ConfigMap
				err := k8sClient.Get(ctx, configMapLookupKey, &fluentBitCm)
				if err != nil {
					return err.Error()
				}
				actualFluentBitConfig := strings.TrimRight(fluentBitCm.Data[cmFileName], "\n")
				return actualFluentBitConfig
			}, timeout, interval).Should(Equal(expectedFluentBitConfig))
		})
		It("Should verify files have been copied into -files configmap", func() {
			Eventually(func() string {
				filesConfigMapLookupKey := types.NamespacedName{
					Name:      testLogPipelineConfig.FilesConfigMap.Name,
					Namespace: testLogPipelineConfig.FilesConfigMap.Namespace,
				}
				var filesCm corev1.ConfigMap
				err := k8sClient.Get(ctx, filesConfigMapLookupKey, &filesCm)
				if err != nil {
					return err.Error()
				}
				return filesCm.Data["myFile"]
			}, timeout, interval).Should(Equal("file-content"))
		})

		It("Should have created flunent-bit parsers configmap", func() {
			Eventually(func() string {
				parserCmName := fmt.Sprintf("%s-parsers", testLogPipelineConfig.DaemonSet.Name)
				parserConfigMapLookupKey := types.NamespacedName{
					Name:      parserCmName,
					Namespace: testLogPipelineConfig.FilesConfigMap.Namespace,
				}
				var parserCm corev1.ConfigMap
				err := k8sClient.Get(ctx, parserConfigMapLookupKey, &parserCm)
				if err != nil {
					return err.Error()
				}
				return parserCm.Data["parsers.conf"]
			}, timeout, interval).Should(Equal(""))
		})

		It("Should verify secret reference is copied into environment secret", func() {
			Eventually(func() string {
				envSecretLookupKey := types.NamespacedName{
					Name:      testLogPipelineConfig.EnvSecret.Name,
					Namespace: testLogPipelineConfig.EnvSecret.Namespace,
				}
				var envSecret corev1.Secret
				err := k8sClient.Get(ctx, envSecretLookupKey, &envSecret)
				if err != nil {
					return err.Error()
				}
				return string(envSecret.Data["myKey"])
			}, timeout, interval).Should(Equal("value"))
		})
		It("Should have added the finalizers", func() {
			Eventually(func() []string {
				loggingConfigLookupKey := types.NamespacedName{
					Name:      LogPipelineName,
					Namespace: testLogPipelineConfig.DaemonSet.Namespace,
				}
				var updatedLogPipeline telemetryv1alpha1.LogPipeline
				err := k8sClient.Get(ctx, loggingConfigLookupKey, &updatedLogPipeline)
				if err != nil {
					return []string{err.Error()}
				}
				return updatedLogPipeline.Finalizers
			}, timeout, interval).Should(ContainElement("FLUENT_BIT_SECTIONS_CONFIG_MAP"))
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
		It("Should have the checksum annotation set to the fluent-bit daemonset", func() {
			// Fluent Bit daemon set should have checksum annotation set
			Eventually(func() bool {
				var fluentBitDaemonSet appsv1.DaemonSet
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testLogPipelineConfig.DaemonSet.Name,
					Namespace: testLogPipelineConfig.DaemonSet.Namespace,
				}, &fluentBitDaemonSet)
				if err != nil {
					return false
				}

				_, found := fluentBitDaemonSet.Spec.Template.Annotations["checksum/logpipeline-config"]
				return found
			}, timeout, interval).Should(BeTrue())
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

	Context("When deleting the log pipeline", Ordered, func() {

		BeforeAll(func() {
			Expect(k8sClient.Delete(ctx, logPipeline)).Should(Succeed())
		})

		It("Should reset the telemetry_all_logpipelines metric", func() {
			Eventually(func() float64 {
				resp, err := http.Get("http://localhost:8080/metrics")
				if err != nil {
					return 0
				}
				var parser expfmt.TextParser
				mf, err := parser.TextToMetricFamilies(resp.Body)
				if err != nil {
					return 0
				}

				return *mf["telemetry_all_logpipelines"].Metric[0].Gauge.Value
			}, timeout, interval).Should(Equal(0.0))
		})

		It("Should reset the telemetry_unsupported_logpipelines metric", func() {
			Eventually(func() float64 {
				resp, err := http.Get("http://localhost:8080/metrics")
				if err != nil {
					return 0
				}
				var parser expfmt.TextParser
				mf, err := parser.TextToMetricFamilies(resp.Body)
				if err != nil {
					return 0
				}

				return *mf["telemetry_unsupported_logpipelines"].Metric[0].Gauge.Value
			}, timeout, interval).Should(Equal(0.0))
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
