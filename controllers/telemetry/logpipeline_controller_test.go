package telemetry

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/common/expfmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
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
	var expectedFluentBitConfig = `[INPUT]
    name             tail
    alias            log-pipeline
    db               /data/flb_log-pipeline.db
    exclude_path     /var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log
    mem_buf_limit    5MB
    multiline.parser docker, cri, go, python, java
    path             /var/log/containers/*_*_*-*.log
    read_from_head   true
    skip_long_lines  on
    storage.type     filesystem
    tag              log-pipeline.*

[FILTER]
    name   record_modifier
    match  log-pipeline.*
    record cluster_identifier ${KUBERNETES_SERVICE_HOST}

[FILTER]
    name                kubernetes
    match               log-pipeline.*
    annotations         off
    buffer_size         1MB
    k8s-logging.exclude off
    k8s-logging.parser  on
    kube_tag_prefix     log-pipeline.var.log.containers.
    labels              on
    merge_log           on

[FILTER]
    name  grep
    match log-pipeline.*
    regex $kubernetes['labels']['app'] my-deployment

[OUTPUT]
    name                     stdout
    match                    log-pipeline.*
    alias                    log-pipeline-stdout
    retry_limit              300
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
			}, timeout, interval).Should(BeTrue())
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
		It("Should verify files have been copied into fluent-bit-files configmap", func() {
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

		It("Should have created fluent-bit-parsers configmap", func() {
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

	Context("When deleting the LogPipeline", Ordered, func() {

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

	Context("When another LogPipeline with missing secret reference is created, then config is not updated", Ordered, func() {
		var pipelineSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			},

			Data: map[string][]byte{
				"host":     []byte("http://foo.bar"),
				"user":     []byte("user"),
				"password": []byte("pass"),
			},
			StringData: nil,
			Type:       "opaque",
		}

		var healthyLogPipeline = &telemetryv1alpha1.LogPipeline{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "telemetry.kyma-project.io/v1alpha1",
				Kind:       "LogPipeline",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "healthy-logpipeline",
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Input: telemetryv1alpha1.Input{Application: telemetryv1alpha1.ApplicationInput{
					Namespaces: telemetryv1alpha1.InputNamespaces{
						System: true}}},
				Output: telemetryv1alpha1.Output{Custom: FluentBitOutputConfig},
			},
		}

		var missingSecretRefLogPipeline = &telemetryv1alpha1.LogPipeline{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "telemetry.kyma-project.io/v1alpha1",
				Kind:       "LogPipeline",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "missing-secret-ref-logpipeline",
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Output: telemetryv1alpha1.Output{
					HTTP: &telemetryv1alpha1.HTTPOutput{
						Host: telemetryv1alpha1.ValueType{
							ValueFrom: &telemetryv1alpha1.ValueFromSource{
								SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
									Name:      "foo",
									Namespace: "default",
									Key:       "host",
								},
							},
						},
						User: telemetryv1alpha1.ValueType{
							ValueFrom: &telemetryv1alpha1.ValueFromSource{
								SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
									Name:      "foo",
									Namespace: "default",
									Key:       "user",
								},
							},
						},
						Password: telemetryv1alpha1.ValueType{
							ValueFrom: &telemetryv1alpha1.ValueFromSource{
								SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
									Name:      "foo",
									Namespace: "default",
									Key:       "password",
								},
							},
						},
						Dedot: false,
					},
				},
			},
		}

		var expectedKeyNotFoundError = errors.New("did not find the key: missing-secret-ref-logpipeline.conf")

		It("Creates a healthy LogPipeline", func() {
			Expect(k8sClient.Create(ctx, healthyLogPipeline)).Should(Succeed())
		})

		It("Creates a LogPipeline with missing secret reference", func() {
			Expect(k8sClient.Create(ctx, missingSecretRefLogPipeline)).Should(Succeed())
		})

		It("Should create a fluent-bit-sections configmap which contains the healthy pipeline", func() {
			Eventually(func() error {
				return validateKeyExistsInFluentbitSectionsConf(ctx, "healthy-logpipeline.conf")
			}, timeout, interval).Should(BeNil())
		})

		It("Should not include the LogPipeline with missing secret in fluent-bit-sections configmap", func() {
			Consistently(func() error {
				return validateKeyExistsInFluentbitSectionsConf(ctx, "missing-secret-ref-logpipeline.conf")
			}, timeout, interval).Should(Equal(expectedKeyNotFoundError))
		})

		It("Should update fluent-bit-sections configmap when secret is created", func() {
			Expect(k8sClient.Create(ctx, pipelineSecret)).Should(Succeed())
			Eventually(func() error {
				return validateKeyExistsInFluentbitSectionsConf(ctx, "missing-secret-ref-logpipeline.conf")
			}, timeout, interval).Should(BeNil())
		})

		It("Should remove LogPipeline from configmap when secret is deleted", func() {
			Expect(k8sClient.Delete(ctx, pipelineSecret)).Should(Succeed())
			Eventually(func() error {
				return validateKeyExistsInFluentbitSectionsConf(ctx, "missing-secret-ref-logpipeline.conf")
			}, timeout, interval).Should(Equal(expectedKeyNotFoundError))
		})

	})

	Context("When creating a LogPipeline with Loki Output", Ordered, func() {
		pipelineName := "pipeline-with-loki-output"
		pipelineWithLokiOutput := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: pipelineName,
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Output: telemetryv1alpha1.Output{
					Loki: &telemetryv1alpha1.LokiOutput{
						URL: telemetryv1alpha1.ValueType{
							Value: "http://logging-loki:3100/loki/api/v1/push",
						},
					},
				}},
		}

		BeforeAll(func() {
			Expect(k8sClient.Create(ctx, pipelineWithLokiOutput)).Should(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, pipelineWithLokiOutput)).Should(Succeed())
			})
		})

		It("Should not include the LogPipeline in fluent-bit-sections configmap", func() {
			expectedErr := errors.New("did not find the key: pipeline-with-loki-output.conf")
			Consistently(func() error {
				return validateKeyExistsInFluentbitSectionsConf(ctx, "pipeline-with-loki-output.conf")
			}, timeout, interval).Should(Equal(expectedErr))
		})

		It("Should have a pending LogPipeline", func() {
			Consistently(func(g Gomega) {
				var pipeline telemetryv1alpha1.LogPipeline
				key := types.NamespacedName{Name: pipelineName}
				g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypeRunning)).To(BeFalse())
			}, timeout, interval).Should(Succeed())
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

func validateKeyExistsInFluentbitSectionsConf(ctx context.Context, key string) error {
	configMapLookupKey := types.NamespacedName{
		Name:      testLogPipelineConfig.SectionsConfigMap.Name,
		Namespace: testLogPipelineConfig.SectionsConfigMap.Namespace,
	}
	var fluentBitCm corev1.ConfigMap
	err := k8sClient.Get(ctx, configMapLookupKey, &fluentBitCm)
	if err != nil {

		return fmt.Errorf("could not get configmap: %s: %s", configMapLookupKey.Name, err)
	}
	if _, ok := fluentBitCm.Data[key]; !ok {
		return fmt.Errorf("did not find the key: %s", key)
	}
	return nil
}
