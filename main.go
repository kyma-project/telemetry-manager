/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	k8sWebhook "sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	operatorcontrollers "github.com/kyma-project/telemetry-manager/controllers/operator"
	telemetrycontrollers "github.com/kyma-project/telemetry-manager/controllers/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	"github.com/kyma-project/telemetry-manager/internal/kubernetes"
	"github.com/kyma-project/telemetry-manager/internal/logger"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logparser"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/metricpipeline"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline"
	"github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector/agent"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector/gateway"
	"github.com/kyma-project/telemetry-manager/internal/webhookcert"
	"github.com/kyma-project/telemetry-manager/webhook/dryrun"
	logparserwebhook "github.com/kyma-project/telemetry-manager/webhook/logparser"
	logpipelinewebhook "github.com/kyma-project/telemetry-manager/webhook/logpipeline"
	logpipelinevalidation "github.com/kyma-project/telemetry-manager/webhook/logpipeline/validation"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	//nolint:gosec // pprof package is required for performance analysis.
	_ "net/http/pprof"
	//nolint:gci // Mandatory kubebuilder imports scaffolding.
	//+kubebuilder:scaffold:imports
)

var (
	certDir                string
	deniedFilterPlugins    string
	deniedOutputPlugins    string
	enableLogging          bool
	enableTracing          bool
	enableMetrics          bool
	logLevel               string
	scheme                 = runtime.NewScheme()
	setupLog               = ctrl.Log.WithName("setup")
	dynamicLoglevel        = zap.NewAtomicLevel()
	configureLogLevelOnFly *logger.LogLevel
	telemetryNamespace     string

	maxLogPipelines    int
	maxTracePipelines  int
	maxMetricPipelines int

	traceGatewayImage                string
	traceGatewayPriorityClass        string
	traceGatewayCPULimit             string
	traceGatewayDynamicCPULimit      string
	traceGatewayMemoryLimit          string
	traceGatewayDynamicMemoryLimit   string
	traceGatewayCPURequest           string
	traceGatewayDynamicCPURequest    string
	traceGatewayMemoryRequest        string
	traceGatewayDynamicMemoryRequest string

	fluentBitMemoryBufferLimit         string
	fluentBitFsBufferLimit             string
	fluentBitCPULimit                  string
	fluentBitMemoryLimit               string
	fluentBitCPURequest                string
	fluentBitMemoryRequest             string
	fluentBitImageVersion              string
	fluentBitExporterVersion           string
	fluentBitConfigPrepperImageVersion string
	fluentBitPriorityClassName         string

	metricGatewayImage                string
	metricGatewayPriorityClass        string
	metricGatewayCPULimit             string
	metricGatewayDynamicCPULimit      string
	metricGatewayMemoryLimit          string
	metricGatewayDynamicMemoryLimit   string
	metricGatewayCPURequest           string
	metricGatewayDynamicCPURequest    string
	metricGatewayMemoryRequest        string
	metricGatewayDynamicMemoryRequest string

	enableTelemetryManagerModule bool
	enableWebhook                bool
	mutex                        sync.Mutex
)

const (
	otelImage              = "europe-docker.pkg.dev/kyma-project/prod/tpi/otel-collector:0.83.0-da21e9f9"
	overridesConfigMapName = "telemetry-override-config"
	fluentBitImage         = "europe-docker.pkg.dev/kyma-project/prod/tpi/fluent-bit:2.1.8-da21e9f9"
	fluentBitExporterImage = "europe-docker.pkg.dev/kyma-project/prod/directory-size-exporter:v20230824-2d68935f"

	fluentBitDaemonSet = "telemetry-fluent-bit"
	webhookServiceName = "telemetry-operator-webhook"

	metricGateway         = "telemetry-metric-gateway"
	metricOTLPServiceName = "telemetry-otlp-metrics"

	traceOTLPServiceName = "telemetry-otlp-traces"
)

//nolint:gochecknoinits // Runtime's scheme addition is required.
func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(telemetryv1alpha1.AddToScheme(scheme))
	utilruntime.Must(operatorv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func getEnvOrDefault(envVar string, defaultValue string) string {
	if value, ok := os.LookupEnv(envVar); ok {
		return value
	}
	return defaultValue
}

//+kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=logpipelines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=logpipelines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=logpipelines/finalizers,verbs=update
//+kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=logparsers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=logparsers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=logparsers/finalizers,verbs=update
//+kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=tracepipelines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=tracepipelines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=metricpipelines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=metricpipelines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=metricpipelines/finalizers,verbs=update

//+kubebuilder:rbac:groups=operator.kyma-project.io,namespace=system,resources=telemetries,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.kyma-project.io,namespace=system,resources=telemetries/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.kyma-project.io,namespace=system,resources=telemetries/finalizers,verbs=update
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=telemetries,verbs=get;list;watch

//+kubebuilder:rbac:groups="",namespace=system,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",namespace=system,resources=services,verbs=get;list;watch;create;update;patch;delete

//+kubebuilder:rbac:groups="",namespace=system,resources=secrets,verbs=create;update;patch;delete
//+kubebuilder:rbac:groups="",namespace=system,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=nodes/metrics,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=nodes/stats,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch
//+kubebuilder:rbac:urls=/metrics,verbs=get
//+kubebuilder:rbac:urls=/metrics/cadvisor,verbs=get

//+kubebuilder:rbac:groups=apps,namespace=system,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,namespace=system,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch

//+kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;create;update;patch;delete

//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete

// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.k8s.io,namespace=system,resources=networkpolicies,verbs=create;update;patch;delete

// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func main() {
	flag.BoolVar(&enableLogging, "enable-logging", true, "Enable configurable logging.")
	flag.BoolVar(&enableTracing, "enable-tracing", true, "Enable configurable tracing.")
	flag.BoolVar(&enableMetrics, "enable-metrics", true, "Enable configurable metrics.")
	flag.StringVar(&logLevel, "log-level", getEnvOrDefault("APP_LOG_LEVEL", "debug"), "Log level (debug, info, warn, error, fatal)")
	flag.StringVar(&certDir, "cert-dir", ".", "Webhook TLS certificate directory")
	flag.StringVar(&telemetryNamespace, "manager-namespace", getEnvOrDefault("MY_POD_NAMESPACE", "default"), "Namespace of the manager")

	flag.StringVar(&traceGatewayImage, "trace-collector-image", otelImage, "Image for tracing OpenTelemetry Collector")
	flag.StringVar(&traceGatewayPriorityClass, "trace-collector-priority-class", "", "Priority class name for tracing OpenTelemetry Collector")
	flag.StringVar(&traceGatewayCPULimit, "trace-collector-cpu-limit", "900m", "CPU limit for tracing OpenTelemetry Collector")
	flag.StringVar(&traceGatewayDynamicCPULimit, "trace-collector-dynamic-cpu-limit", "100m", "Additional CPU limit for tracing OpenTelemetry Collector per TracePipeline")
	flag.StringVar(&traceGatewayMemoryLimit, "trace-collector-memory-limit", "512Mi", "Memory limit for tracing OpenTelemetry Collector")
	flag.StringVar(&traceGatewayDynamicMemoryLimit, "trace-collector-dynamic-memory-limit", "512Mi", "Additional memory limit for tracing OpenTelemetry Collector per TracePipeline")
	flag.StringVar(&traceGatewayCPURequest, "trace-collector-cpu-request", "25m", "CPU request for tracing OpenTelemetry Collector")
	flag.StringVar(&traceGatewayDynamicCPURequest, "trace-collector-dynamic-cpu-request", "0", "Additional CPU request for tracing OpenTelemetry Collector per TracePipeline")
	flag.StringVar(&traceGatewayMemoryRequest, "trace-collector-memory-request", "32Mi", "Memory request for tracing OpenTelemetry Collector")
	flag.StringVar(&traceGatewayDynamicMemoryRequest, "trace-collector-dynamic-memory-request", "0", "Additional memory request for tracing OpenTelemetry Collector per TracePipeline")
	flag.IntVar(&maxTracePipelines, "trace-collector-pipelines", 3, "Maximum number of TracePipelines to be created. If 0, no limit is applied.")

	flag.StringVar(&metricGatewayImage, "metric-gateway-image", otelImage, "Image for metrics OpenTelemetry Collector")
	flag.StringVar(&metricGatewayPriorityClass, "metric-gateway-priority-class", "", "Priority class name for metrics OpenTelemetry Collector")
	flag.StringVar(&metricGatewayCPULimit, "metric-gateway-cpu-limit", "900m", "CPU limit for metrics OpenTelemetry Collector")
	flag.StringVar(&metricGatewayDynamicCPULimit, "metric-gateway-dynamic-cpu-limit", "100m", "Additional CPU limit for metrics OpenTelemetry Collector per MetricPipeline")
	flag.StringVar(&metricGatewayMemoryLimit, "metric-gateway-memory-limit", "512Mi", "Memory limit for metrics OpenTelemetry Collector")
	flag.StringVar(&metricGatewayDynamicMemoryLimit, "metric-gateway-dynamic-memory-limit", "512Mi", "Additional memory limit for metrics OpenTelemetry Collector per MetricPipeline")
	flag.StringVar(&metricGatewayCPURequest, "metric-gateway-cpu-request", "25m", "CPU request for metrics OpenTelemetry Collector")
	flag.StringVar(&metricGatewayDynamicCPURequest, "metric-gateway-dynamic-cpu-request", "0", "Additional CPU request for metrics OpenTelemetry Collector per MetricPipeline")
	flag.StringVar(&metricGatewayMemoryRequest, "metric-gateway-memory-request", "32Mi", "Memory request for metrics OpenTelemetry Collector")
	flag.StringVar(&metricGatewayDynamicMemoryRequest, "metric-gateway-dynamic-memory-request", "0", "Additional memory request for metrics OpenTelemetry Collector per MetricPipeline")
	flag.IntVar(&maxMetricPipelines, "metric-gateway-pipelines", 3, "Maximum number of MetricPipelines to be created. If 0, no limit is applied.")

	flag.StringVar(&fluentBitMemoryBufferLimit, "fluent-bit-memory-buffer-limit", "10M", "Fluent Bit memory buffer limit per log pipeline")
	flag.StringVar(&fluentBitFsBufferLimit, "fluent-bit-filesystem-buffer-limit", "1G", "Fluent Bit filesystem buffer limit per log pipeline")
	flag.StringVar(&deniedFilterPlugins, "fluent-bit-denied-filter-plugins", "kubernetes,rewrite_tag,multiline", "Comma separated list of denied filter plugins even if allowUnsupportedPlugins is enabled. If empty, all filter plugins are allowed.")
	flag.StringVar(&fluentBitCPULimit, "fluent-bit-cpu-limit", "1", "CPU limit for tracing fluent-bit")
	flag.StringVar(&fluentBitMemoryLimit, "fluent-bit-memory-limit", "1Gi", "Memory limit for fluent-bit")
	flag.StringVar(&fluentBitCPURequest, "fluent-bit-cpu-request", "100m", "CPU request for fluent-bit")
	flag.StringVar(&fluentBitMemoryRequest, "fluent-bit-memory-request", "50Mi", "Memory request for fluent-bit")
	flag.StringVar(&fluentBitImageVersion, "fluent-bit-image", fluentBitImage, "Image for fluent-bit")
	flag.StringVar(&fluentBitExporterVersion, "fluent-bit-exporter-image", fluentBitExporterImage, "Image for exporting fluent bit filesystem usage")
	flag.StringVar(&fluentBitPriorityClassName, "fluent-bit-priority-class-name", "", "Name of the priority class of fluent bit ")

	flag.StringVar(&deniedOutputPlugins, "fluent-bit-denied-output-plugins", "", "Comma separated list of denied output plugins even if allowUnsupportedPlugins is enabled. If empty, all output plugins are allowed.")
	flag.IntVar(&maxLogPipelines, "fluent-bit-max-pipelines", 5, "Maximum number of LogPipelines to be created. If 0, no limit is applied.")

	flag.BoolVar(&enableWebhook, "validating-webhook-enabled", false, "Create validating webhook for LogPipelines and LogParsers.")

	flag.BoolVar(&enableTelemetryManagerModule, "enable-telemetry-manager-module", true, "Enable telemetry manager.")

	flag.Parse()
	if err := validateFlags(); err != nil {
		setupLog.Error(err, "Invalid flag provided")
		os.Exit(1)
	}

	parsedLevel, err := zapcore.ParseLevel(logLevel)
	if err != nil {
		os.Exit(1)
	}
	dynamicLoglevel.SetLevel(parsedLevel)
	configureLogLevelOnFly = logger.NewLogReconfigurer(dynamicLoglevel)

	ctrLogger, err := logger.New("json", logLevel, dynamicLoglevel)

	go func() {
		server := &http.Server{
			Addr:              ":6060",
			ReadHeaderTimeout: 10 * time.Second,
		}

		err := server.ListenAndServe()
		if err != nil {
			mutex.Lock()
			setupLog.Error(err, "Cannot start pprof server")
			mutex.Unlock()
		}
	}()

	ctrl.SetLogger(zapr.NewLogger(ctrLogger.WithContext().Desugar()))
	if err != nil {
		os.Exit(1)
	}
	defer func() {
		if err = ctrLogger.WithContext().Sync(); err != nil {
			setupLog.Error(err, "Failed to flush logger")
		}
	}()

	syncPeriod := 1 * time.Hour
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		Metrics:                 metricsserver.Options{BindAddress: ":8080"},
		HealthProbeBindAddress:  ":8081",
		LeaderElection:          true,
		LeaderElectionNamespace: telemetryNamespace,
		LeaderElectionID:        "cdd7ef0b.kyma-project.io",
		WebhookServer: k8sWebhook.NewServer(k8sWebhook.Options{
			Port:    9443,
			CertDir: certDir,
		}),
		Cache: cache.Options{
			SyncPeriod: &syncPeriod,
			ByObject: map[client.Object]cache.ByObject{
				&appsv1.Deployment{}:          {Field: setNamespaceFieldSelector()},
				&appsv1.ReplicaSet{}:          {Field: setNamespaceFieldSelector()},
				&appsv1.DaemonSet{}:           {Field: setNamespaceFieldSelector()},
				&corev1.ConfigMap{}:           {Field: setNamespaceFieldSelector()},
				&corev1.ServiceAccount{}:      {Field: setNamespaceFieldSelector()},
				&corev1.Service{}:             {Field: setNamespaceFieldSelector()},
				&networkingv1.NetworkPolicy{}: {Field: setNamespaceFieldSelector()},
			},
		},
	})

	if err != nil {
		setupLog.Error(err, "Failed to start manager")
		os.Exit(1)
	}

	if enableLogging {
		setupLog.Info("Starting with logging controllers")

		mgr.GetWebhookServer().Register("/validate-logpipeline", &k8sWebhook.Admission{Handler: createLogPipelineValidator(mgr.GetClient())})
		mgr.GetWebhookServer().Register("/validate-logparser", &k8sWebhook.Admission{Handler: createLogParserValidator(mgr.GetClient())})

		if err = createLogPipelineReconciler(mgr.GetClient()).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Failed to create controller", "controller", "LogPipeline")
			os.Exit(1)
		}

		if err = createLogParserReconciler(mgr.GetClient()).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Failed to create controller", "controller", "LogParser")
			os.Exit(1)
		}

	}

	if enableTracing {
		setupLog.Info("Starting with tracing controller")
		if err = createTracePipelineReconciler(mgr.GetClient()).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Failed to create controller", "controller", "TracePipeline")
			os.Exit(1)
		}
	}

	if enableMetrics {
		setupLog.Info("Starting with metrics controller")
		if err = createMetricPipelineReconciler(mgr.GetClient()).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Failed to create controller", "controller", "MetricPipeline")
			os.Exit(1)
		}
	}

	webhookConfig := createWebhookConfig()

	if enableTelemetryManagerModule {
		setupLog.Info("Starting with telemetry manager controller")

		if err = createTelemetryReconciler(mgr.GetClient(), mgr.GetScheme(), webhookConfig).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Telemetry")
			os.Exit(1)
		}
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", mgr.GetWebhookServer().StartedChecker()); err != nil {
		setupLog.Error(err, "Failed to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", mgr.GetWebhookServer().StartedChecker()); err != nil {
		setupLog.Error(err, "Failed to set up ready check")
		os.Exit(1)
	}

	if enableWebhook {
		// Create own client since manager might not be started while using
		clientOptions := client.Options{
			Scheme: scheme,
		}
		k8sClient, err := client.New(mgr.GetConfig(), clientOptions)
		if err != nil {
			setupLog.Error(err, "Failed to create client")
			os.Exit(1)
		}

		if err = webhookcert.EnsureCertificate(context.Background(), k8sClient, webhookConfig.CertConfig); err != nil {
			setupLog.Error(err, "Failed to ensure webhook cert")
			os.Exit(1)
		}
		setupLog.Info("Ensured webhook cert")

		// Temporary solution for non-modularized telemetry operator
		if !enableTelemetryManagerModule {
			go func() {
				for range time.Tick(1 * time.Hour) {
					if ensureErr := webhookcert.EnsureCertificate(context.Background(), k8sClient, webhookConfig.CertConfig); ensureErr != nil {
						setupLog.Error(ensureErr, "Failed to ensure webhook cert")
					}
					setupLog.Info("Ensured webhook cert")
				}
			}()
		}
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Failed to run manager")
		os.Exit(1)
	}
}

func setNamespaceFieldSelector() fields.Selector {
	return fields.SelectorFromSet(fields.Set{"metadata.namespace": telemetryNamespace})
}

func validateFlags() error {

	if logLevel != "debug" && logLevel != "info" && logLevel != "warn" && logLevel != "error" && logLevel != "fatal" {
		return errors.New("--log-level has to be one of debug, info, warn, error, fatal")
	}
	return nil
}

func createLogPipelineReconciler(client client.Client) *telemetrycontrollers.LogPipelineReconciler {
	config := logpipeline.Config{
		SectionsConfigMap: types.NamespacedName{Name: "telemetry-fluent-bit-sections", Namespace: telemetryNamespace},
		FilesConfigMap:    types.NamespacedName{Name: "telemetry-fluent-bit-files", Namespace: telemetryNamespace},
		LuaConfigMap:      types.NamespacedName{Name: "telemetry-fluent-bit-luascripts", Namespace: telemetryNamespace},
		ParsersConfigMap:  types.NamespacedName{Name: "telemetry-fluent-bit-parsers", Namespace: telemetryNamespace},
		EnvSecret:         types.NamespacedName{Name: "telemetry-fluent-bit-env", Namespace: telemetryNamespace},
		DaemonSet:         types.NamespacedName{Name: fluentBitDaemonSet, Namespace: telemetryNamespace},
		OverrideConfigMap: types.NamespacedName{Name: overridesConfigMapName, Namespace: telemetryNamespace},
		PipelineDefaults:  createPipelineDefaults(),
		DaemonSetConfig: fluentbit.DaemonSetConfig{
			FluentBitImage:              fluentBitImageVersion,
			FluentBitConfigPrepperImage: fluentBitConfigPrepperImageVersion,
			ExporterImage:               fluentBitExporterVersion,
			PriorityClassName:           fluentBitPriorityClassName,
			CPULimit:                    resource.MustParse(fluentBitCPULimit),
			MemoryLimit:                 resource.MustParse(fluentBitMemoryLimit),
			CPURequest:                  resource.MustParse(fluentBitCPURequest),
			MemoryRequest:               resource.MustParse(fluentBitMemoryRequest),
		},
	}
	overridesHandler := overrides.New(configureLogLevelOnFly, &kubernetes.ConfigmapProber{Client: client})

	return telemetrycontrollers.NewLogPipelineReconciler(
		client,
		logpipeline.NewReconciler(client, config, &kubernetes.DaemonSetProber{Client: client}, overridesHandler),
		config)
}

func createLogParserReconciler(client client.Client) *telemetrycontrollers.LogParserReconciler {
	config := logparser.Config{
		ParsersConfigMap: types.NamespacedName{Name: "telemetry-fluent-bit-parsers", Namespace: telemetryNamespace},
		DaemonSet:        types.NamespacedName{Name: fluentBitDaemonSet, Namespace: telemetryNamespace},
	}
	overridesHandler := overrides.New(configureLogLevelOnFly, &kubernetes.ConfigmapProber{Client: client})

	return telemetrycontrollers.NewLogParserReconciler(
		client,
		logparser.NewReconciler(
			client,
			config,
			&kubernetes.DaemonSetProber{Client: client},
			&kubernetes.DaemonSetAnnotator{Client: client},
			overridesHandler,
		),
		config,
	)
}

func createLogPipelineValidator(client client.Client) *logpipelinewebhook.ValidatingWebhookHandler {
	return logpipelinewebhook.NewValidatingWebhookHandler(
		client,
		logpipelinevalidation.NewVariablesValidator(client),
		logpipelinevalidation.NewMaxPipelinesValidator(maxLogPipelines),
		logpipelinevalidation.NewFilesValidator(),
		admission.NewDecoder(scheme),
		dryrun.NewDryRunner(client, createDryRunConfig()),
		&telemetryv1alpha1.LogPipelineValidationConfig{DeniedOutPutPlugins: parsePlugins(deniedOutputPlugins), DeniedFilterPlugins: parsePlugins(deniedFilterPlugins)})
}

func createLogParserValidator(client client.Client) *logparserwebhook.ValidatingWebhookHandler {
	return logparserwebhook.NewValidatingWebhookHandler(
		client,
		dryrun.NewDryRunner(client, createDryRunConfig()))
}

func createTracePipelineReconciler(client client.Client) *telemetrycontrollers.TracePipelineReconciler {
	config := tracepipeline.Config{
		Gateway: gateway.Config{
			Namespace: telemetryNamespace,
			BaseName:  "telemetry-trace-collector",
			Deployment: gateway.DeploymentConfig{
				Image:                traceGatewayImage,
				PriorityClassName:    traceGatewayPriorityClass,
				BaseCPULimit:         resource.MustParse(traceGatewayCPULimit),
				DynamicCPULimit:      resource.MustParse(traceGatewayDynamicCPULimit),
				BaseMemoryLimit:      resource.MustParse(traceGatewayMemoryLimit),
				DynamicMemoryLimit:   resource.MustParse(traceGatewayDynamicMemoryLimit),
				BaseCPURequest:       resource.MustParse(traceGatewayCPURequest),
				DynamicCPURequest:    resource.MustParse(traceGatewayDynamicCPURequest),
				BaseMemoryRequest:    resource.MustParse(traceGatewayMemoryRequest),
				DynamicMemoryRequest: resource.MustParse(traceGatewayDynamicMemoryRequest),
			},
			Service: gateway.ServiceConfig{
				OTLPServiceName: traceOTLPServiceName,
			},
		},
		OverridesConfigMapName: types.NamespacedName{Name: overridesConfigMapName, Namespace: telemetryNamespace},
		MaxPipelines:           maxTracePipelines,
	}
	overridesHandler := overrides.New(configureLogLevelOnFly, &kubernetes.ConfigmapProber{Client: client})

	return telemetrycontrollers.NewTracePipelineReconciler(
		client,
		tracepipeline.NewReconciler(client, config, &kubernetes.DeploymentProber{Client: client}, overridesHandler),
	)
}

func createMetricPipelineReconciler(client client.Client) *telemetrycontrollers.MetricPipelineReconciler {
	config := metricpipeline.Config{
		Agent: agent.Config{
			Namespace: telemetryNamespace,
			BaseName:  "telemetry-metric-agent",
			DaemonSet: agent.DaemonSetConfig{
				Image:             metricGatewayImage,
				PriorityClassName: metricGatewayPriorityClass,
				CPULimit:          resource.MustParse("1"),
				MemoryLimit:       resource.MustParse("1Gi"),
				CPURequest:        resource.MustParse("15m"),
				MemoryRequest:     resource.MustParse("50Mi"),
			},
		},
		Gateway: gateway.Config{
			Namespace: telemetryNamespace,
			BaseName:  "telemetry-metric-gateway",
			Deployment: gateway.DeploymentConfig{
				Image:                metricGatewayImage,
				PriorityClassName:    metricGatewayPriorityClass,
				BaseCPULimit:         resource.MustParse(metricGatewayCPULimit),
				DynamicCPULimit:      resource.MustParse(metricGatewayDynamicCPULimit),
				BaseMemoryLimit:      resource.MustParse(metricGatewayMemoryLimit),
				DynamicMemoryLimit:   resource.MustParse(metricGatewayDynamicMemoryLimit),
				BaseCPURequest:       resource.MustParse(metricGatewayCPURequest),
				DynamicCPURequest:    resource.MustParse(metricGatewayDynamicCPURequest),
				BaseMemoryRequest:    resource.MustParse(metricGatewayMemoryRequest),
				DynamicMemoryRequest: resource.MustParse(metricGatewayDynamicMemoryRequest),
			},
			Service: gateway.ServiceConfig{
				OTLPServiceName: metricOTLPServiceName,
			},
		},
		OverridesConfigMapName: types.NamespacedName{Name: overridesConfigMapName, Namespace: telemetryNamespace},
		MaxPipelines:           maxMetricPipelines,
	}

	overridesHandler := overrides.New(configureLogLevelOnFly, &kubernetes.ConfigmapProber{Client: client})

	return telemetrycontrollers.NewMetricPipelineReconciler(
		client,
		metricpipeline.NewReconciler(client, config, &kubernetes.DeploymentProber{Client: client}, overridesHandler))
}

func createDryRunConfig() dryrun.Config {
	return dryrun.Config{
		FluentBitConfigMapName: types.NamespacedName{Name: "telemetry-fluent-bit", Namespace: telemetryNamespace},
		PipelineDefaults:       createPipelineDefaults(),
	}
}

func createPipelineDefaults() builder.PipelineDefaults {
	return builder.PipelineDefaults{
		InputTag:          "tele",
		MemoryBufferLimit: fluentBitMemoryBufferLimit,
		StorageType:       "filesystem",
		FsBufferLimit:     fluentBitFsBufferLimit,
	}
}

func parsePlugins(s string) []string {
	return strings.SplitN(strings.ReplaceAll(s, " ", ""), ",", len(s))
}

func createTelemetryReconciler(client client.Client, scheme *runtime.Scheme, webhookConfig telemetry.WebhookConfig) *operatorcontrollers.TelemetryReconciler {
	config := telemetry.Config{
		Traces: telemetry.TracesConfig{
			OTLPServiceName: traceOTLPServiceName,
			Namespace:       telemetryNamespace,
		},
		Metrics: telemetry.MetricsConfig{
			Enabled:         enableMetrics,
			OTLPServiceName: metricOTLPServiceName,
			Namespace:       telemetryNamespace,
		},
		Webhook: webhookConfig,
	}

	return operatorcontrollers.NewTelemetryReconciler(client, telemetry.NewReconciler(client, scheme, config), config)
}

func createWebhookConfig() telemetry.WebhookConfig {
	return telemetry.WebhookConfig{
		Enabled: enableWebhook,
		CertConfig: webhookcert.Config{
			CertDir: certDir,
			ServiceName: types.NamespacedName{
				Name:      webhookServiceName,
				Namespace: telemetryNamespace,
			},
			CASecretName: types.NamespacedName{
				Name:      "telemetry-webhook-cert",
				Namespace: telemetryNamespace,
			},
			WebhookName: types.NamespacedName{
				Name: "validation.webhook.telemetry.kyma-project.io",
			},
		},
	}
}
