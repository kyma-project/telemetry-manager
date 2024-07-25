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
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-logr/zapr"
	"go.uber.org/zap/zapcore"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/controllers/operator"
	telemetrycontrollers "github.com/kyma-project/telemetry-manager/controllers/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	"github.com/kyma-project/telemetry-manager/internal/logger"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/metricpipeline"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/resources/selfmonitor"
	selfmonitorwebhook "github.com/kyma-project/telemetry-manager/internal/selfmonitor/webhook"
	"github.com/kyma-project/telemetry-manager/internal/webhookcert"
	"github.com/kyma-project/telemetry-manager/webhook/dryrun"
	logparserwebhook "github.com/kyma-project/telemetry-manager/webhook/logparser"
	logpipelinewebhook "github.com/kyma-project/telemetry-manager/webhook/logpipeline"
	"github.com/kyma-project/telemetry-manager/webhook/logpipeline/validation"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	//nolint:gosec // pprof package is required for performance analysis.
	//nolint:gci // Mandatory kubebuilder imports scaffolding.
)

var (
	certDir            string
	logLevel           string
	scheme             = runtime.NewScheme()
	setupLog           = ctrl.Log.WithName("setup")
	telemetryNamespace string
	enableWebhook      bool

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

	fluentBitDeniedFilterPlugins string
	fluentBitDeniedOutputPlugins string
	fluentBitMemoryBufferLimit   string
	fluentBitFsBufferLimit       string
	fluentBitCPULimit            string
	fluentBitMemoryLimit         string
	fluentBitCPURequest          string
	fluentBitMemoryRequest       string
	fluentBitImage               string
	fluentBitExporterImage       string
	fluentBitPriorityClassName   string

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

	selfMonitorImage         string
	selfMonitorPriorityClass string

	version = "main"
)

const (
	defaultFluentBitExporterImage = "europe-docker.pkg.dev/kyma-project/prod/directory-size-exporter:v20240605-7743c77e"
	defaultFluentBitImage         = "europe-docker.pkg.dev/kyma-project/prod/tpi/fluent-bit:3.1.3-44a3707"
	defaultOtelImage              = "europe-docker.pkg.dev/kyma-project/prod/kyma-otel-collector:0.104.0-main"
	defaultSelfMonitorImage       = "europe-docker.pkg.dev/kyma-project/prod/tpi/telemetry-self-monitor:2.53.1-729b0b4"

	metricOTLPServiceName = "telemetry-otlp-metrics"
	traceOTLPServiceName  = "telemetry-otlp-traces"
	webhookServiceName    = "telemetry-manager-webhook"

	selfMonitorName = "telemetry-self-monitor"
)

//nolint:gochecknoinits // Runtime's scheme addition is required.
func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))

	utilruntime.Must(telemetryv1alpha1.AddToScheme(scheme))
	utilruntime.Must(operatorv1alpha1.AddToScheme(scheme))
	utilruntime.Must(istiosecurityclientv1.AddToScheme(scheme))
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

//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch

//+kubebuilder:rbac:groups=apps,namespace=system,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,namespace=system,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch

//+kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;create;update;patch;delete

//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete

//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.k8s.io,namespace=system,resources=networkpolicies,verbs=create;update;patch;delete

// +kubebuilder:rbac:groups=security.istio.io,resources=peerauthentications,verbs=get;list;watch
// +kubebuilder:rbac:groups=security.istio.io,namespace=system,resources=peerauthentications,verbs=create;update;patch;delete

// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func main() {
	flag.StringVar(&logLevel, "log-level", getEnvOrDefault("APP_LOG_LEVEL", "debug"), "Log level (debug, info, warn, error, fatal)")
	flag.StringVar(&certDir, "cert-dir", ".", "Webhook TLS certificate directory")
	flag.StringVar(&telemetryNamespace, "manager-namespace", getEnvOrDefault("MY_POD_NAMESPACE", "default"), "Namespace of the manager")
	flag.BoolVar(&enableWebhook, "validating-webhook-enabled", false, "Create validating webhook for LogPipelines and LogParsers.")

	flag.StringVar(&traceGatewayImage, "trace-collector-image", defaultOtelImage, "Image for tracing OpenTelemetry Collector")
	flag.StringVar(&traceGatewayPriorityClass, "trace-collector-priority-class", "", "Priority class name for tracing OpenTelemetry Collector")
	flag.StringVar(&traceGatewayCPULimit, "trace-collector-cpu-limit", "700m", "CPU limit for tracing OpenTelemetry Collector")
	flag.StringVar(&traceGatewayDynamicCPULimit, "trace-collector-dynamic-cpu-limit", "500m", "Additional CPU limit for tracing OpenTelemetry Collector per TracePipeline")
	flag.StringVar(&traceGatewayMemoryLimit, "trace-collector-memory-limit", "500Mi", "Memory limit for tracing OpenTelemetry Collector")
	flag.StringVar(&traceGatewayDynamicMemoryLimit, "trace-collector-dynamic-memory-limit", "1500Mi", "Additional memory limit for tracing OpenTelemetry Collector per TracePipeline")
	flag.StringVar(&traceGatewayCPURequest, "trace-collector-cpu-request", "100m", "CPU request for tracing OpenTelemetry Collector")
	flag.StringVar(&traceGatewayDynamicCPURequest, "trace-collector-dynamic-cpu-request", "100m", "Additional CPU request for tracing OpenTelemetry Collector per TracePipeline")
	flag.StringVar(&traceGatewayMemoryRequest, "trace-collector-memory-request", "32Mi", "Memory request for tracing OpenTelemetry Collector")
	flag.StringVar(&traceGatewayDynamicMemoryRequest, "trace-collector-dynamic-memory-request", "0", "Additional memory request for tracing OpenTelemetry Collector per TracePipeline")
	flag.IntVar(&maxTracePipelines, "trace-collector-pipelines", 3, "Maximum number of TracePipelines to be created. If 0, no limit is applied.")

	flag.StringVar(&metricGatewayImage, "metric-gateway-image", defaultOtelImage, "Image for metrics OpenTelemetry Collector")
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

	flag.StringVar(&fluentBitDeniedFilterPlugins, "fluent-bit-denied-filter-plugins", "kubernetes,rewrite_tag", "Comma separated list of denied filter plugins even if allowUnsupportedPlugins is enabled. If empty, all filter plugins are allowed.")
	flag.StringVar(&fluentBitDeniedOutputPlugins, "fluent-bit-denied-output-plugins", "", "Comma separated list of denied output plugins even if allowUnsupportedPlugins is enabled. If empty, all output plugins are allowed.")
	flag.StringVar(&fluentBitMemoryBufferLimit, "fluent-bit-memory-buffer-limit", "10M", "Fluent Bit memory buffer limit per log pipeline")
	flag.StringVar(&fluentBitFsBufferLimit, "fluent-bit-filesystem-buffer-limit", "1G", "Fluent Bit filesystem buffer limit per log pipeline")
	flag.StringVar(&fluentBitCPULimit, "fluent-bit-cpu-limit", "1", "CPU limit for tracing fluent-bit")
	flag.StringVar(&fluentBitMemoryLimit, "fluent-bit-memory-limit", "1Gi", "Memory limit for fluent-bit")
	flag.StringVar(&fluentBitCPURequest, "fluent-bit-cpu-request", "100m", "CPU request for fluent-bit")
	flag.StringVar(&fluentBitMemoryRequest, "fluent-bit-memory-request", "50Mi", "Memory request for fluent-bit")
	flag.StringVar(&fluentBitImage, "fluent-bit-image", defaultFluentBitImage, "Image for fluent-bit")
	flag.StringVar(&fluentBitExporterImage, "fluent-bit-exporter-image", defaultFluentBitExporterImage, "Image for exporting fluent bit filesystem usage")
	flag.StringVar(&fluentBitPriorityClassName, "fluent-bit-priority-class-name", "", "Name of the priority class of fluent bit ")
	flag.IntVar(&maxLogPipelines, "fluent-bit-max-pipelines", 5, "Maximum number of LogPipelines to be created. If 0, no limit is applied.")

	flag.StringVar(&selfMonitorImage, "self-monitor-image", defaultSelfMonitorImage, "Image for self-monitor")
	flag.StringVar(&selfMonitorPriorityClass, "self-monitor-priority-class", "", "Priority class name for self-monitor")

	flag.Parse()
	if err := validateFlags(); err != nil {
		setupLog.Error(err, "Invalid flag provided")
		os.Exit(1)
	}
	parsedLevel, err := zapcore.ParseLevel(logLevel)
	if err != nil {
		os.Exit(1)
	}
	overrides.AtomicLevel().SetLevel(parsedLevel)
	ctrLogger, err := logger.New(overrides.AtomicLevel())

	ctrl.SetLogger(zapr.NewLogger(ctrLogger.WithContext().Desugar()))
	if err != nil {
		os.Exit(1)
	}

	defer func() {
		if err = ctrLogger.WithContext().Sync(); err != nil {
			setupLog.Error(err, "Failed to flush logger")
		}
	}()

	syncPeriod := 1 * time.Minute
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		Metrics:                 metricsserver.Options{BindAddress: ":8080"},
		HealthProbeBindAddress:  ":8081",
		PprofBindAddress:        ":6060",
		LeaderElection:          true,
		LeaderElectionNamespace: telemetryNamespace,
		LeaderElectionID:        "cdd7ef0b.kyma-project.io",
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    9443,
			CertDir: certDir,
		}),
		Cache: cache.Options{
			SyncPeriod: &syncPeriod,

			// The operator handles various resource that are namespace-scoped, and additionally some resources that are cluster-scoped (clusterroles, clusterrolebindings, etc.).
			// For namespace-scoped resources we want to restrict the operator permissions to only fetch resources from a given namespace.
			ByObject: map[client.Object]cache.ByObject{
				&appsv1.Deployment{}:          {Field: setNamespaceFieldSelector()},
				&appsv1.ReplicaSet{}:          {Field: setNamespaceFieldSelector()},
				&appsv1.DaemonSet{}:           {Field: setNamespaceFieldSelector()},
				&corev1.ConfigMap{}:           {Field: setNamespaceFieldSelector()},
				&corev1.ServiceAccount{}:      {Field: setNamespaceFieldSelector()},
				&corev1.Service{}:             {Field: setNamespaceFieldSelector()},
				&networkingv1.NetworkPolicy{}: {Field: setNamespaceFieldSelector()},
				&corev1.Secret{}:              {Field: setNamespaceFieldSelector()},
				&operatorv1alpha1.Telemetry{}: {Field: setNamespaceFieldSelector()},
			},
		},
		Client: client.Options{
			Cache: &client.CacheOptions{
				DisableFor: []client.Object{
					&corev1.Secret{},
				},
			},
		},
	})

	if err != nil {
		setupLog.Error(err, "Failed to start manager")
		os.Exit(1)
	}

	tracePipelineReconcileTriggerChan := make(chan event.GenericEvent)
	enableTracingController(mgr, tracePipelineReconcileTriggerChan)

	metricPipelineReconcileTriggerChan := make(chan event.GenericEvent)
	enableMetricsController(mgr, metricPipelineReconcileTriggerChan)

	logPipelineReconcileTriggerChan := make(chan event.GenericEvent)
	enableLoggingController(mgr, logPipelineReconcileTriggerChan)

	webhookConfig := createWebhookConfig()
	selfMonitorConfig := createSelfMonitoringConfig()

	enableTelemetryModuleController(mgr, webhookConfig, selfMonitorConfig)

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
		enableWebhookServer(mgr, webhookConfig)
	}

	if enableWebhook {
		mgr.GetWebhookServer().Register("/api/v2/alerts", selfmonitorwebhook.NewHandler(
			mgr.GetClient(),
			selfmonitorwebhook.WithTracePipelineSubscriber(tracePipelineReconcileTriggerChan),
			selfmonitorwebhook.WithMetricPipelineSubscriber(metricPipelineReconcileTriggerChan),
			selfmonitorwebhook.WithLogPipelineSubscriber(logPipelineReconcileTriggerChan),
			selfmonitorwebhook.WithLogger(ctrl.Log.WithName("self-monitor-webhook"))))
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Failed to run manager")
		os.Exit(1)
	}
}

func enableTelemetryModuleController(mgr manager.Manager, webhookConfig telemetry.WebhookConfig, selfMonitorConfig telemetry.SelfMonitorConfig) {
	setupLog.WithValues("version", version).Info("Starting with telemetry manager controller")

	telemetryPipleineController := operator.NewTelemetryController(
		mgr.GetClient(),
		mgr.GetScheme(),
		operator.TelemetryControllerConfig{
			Config: telemetry.Config{
				Traces: telemetry.TracesConfig{
					OTLPServiceName: traceOTLPServiceName,
					Namespace:       telemetryNamespace,
				},
				Metrics: telemetry.MetricsConfig{
					OTLPServiceName: metricOTLPServiceName,
					Namespace:       telemetryNamespace,
				},
				Webhook:     webhookConfig,
				SelfMonitor: selfMonitorConfig,
			},
			SelfMonitorName:    selfMonitorName,
			TelemetryNamespace: telemetryNamespace,
		},
	)

	if err := telemetryPipleineController.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Telemetry")
		os.Exit(1)
	}
}

func enableLoggingController(mgr manager.Manager, reconcileTriggerChan <-chan event.GenericEvent) {
	setupLog.Info("Starting with logging controllers")

	mgr.GetWebhookServer().Register("/validate-logpipeline", &webhook.Admission{Handler: createLogPipelineValidator(mgr.GetClient())})
	mgr.GetWebhookServer().Register("/validate-logparser", &webhook.Admission{Handler: createLogParserValidator(mgr.GetClient())})

	logPipelineController, err := telemetrycontrollers.NewLogPipelineController(
		mgr.GetClient(),
		reconcileTriggerChan,
		telemetrycontrollers.LogPipelineControllerConfig{
			ExporterImage:          fluentBitExporterImage,
			FluentBitCPULimit:      fluentBitCPULimit,
			FluentBitCPURequest:    fluentBitCPURequest,
			FluentBitMemoryLimit:   fluentBitMemoryLimit,
			FluentBitMemoryRequest: fluentBitMemoryRequest,
			FluentBitImage:         fluentBitImage,
			PipelineDefaults:       createPipelineDefaults(),
			PriorityClassName:      fluentBitPriorityClassName,
			SelfMonitorName:        selfMonitorName,
			TelemetryNamespace:     telemetryNamespace,
		},
	)
	if err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "LogPipeline")
		os.Exit(1)
	}
	if err := logPipelineController.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to setup controller", "controller", "LogPipeline")
		os.Exit(1)
	}

	logParserController := telemetrycontrollers.NewLogParserController(
		mgr.GetClient(),
		telemetrycontrollers.LogParserControllerConfig{
			TelemetryNamespace: telemetryNamespace,
		})
	if err := logParserController.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "LogParser")
		os.Exit(1)
	}
}

func enableTracingController(mgr manager.Manager, reconcileTriggerChan <-chan event.GenericEvent) {
	setupLog.Info("Starting with tracing controller")

	tracePipelineController, err := telemetrycontrollers.NewTracePipelineController(
		mgr.GetClient(),
		reconcileTriggerChan,
		telemetrycontrollers.TracePipelineControllerConfig{
			Config: tracepipeline.Config{
				Gateway: otelcollector.GatewayConfig{
					Config: otelcollector.Config{
						Namespace: telemetryNamespace,
						BaseName:  "telemetry-trace-collector",
					},
					Deployment: otelcollector.DeploymentConfig{
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
					OTLPServiceName: traceOTLPServiceName,
				},
				MaxPipelines: maxTracePipelines,
			},
			TelemetryNamespace: telemetryNamespace,
			SelfMonitorName:    selfMonitorName,
		},
	)
	if err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "TracePipeline")
		os.Exit(1)
	}
	if err := tracePipelineController.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to setup controller", "controller", "TracePipeline")
		os.Exit(1)
	}
}

func enableMetricsController(mgr manager.Manager, reconcileTriggerChan <-chan event.GenericEvent) {
	setupLog.Info("Starting with metrics controller")

	metricPipelineController, err := telemetrycontrollers.NewMetricPipelineController(
		mgr.GetClient(),
		reconcileTriggerChan,
		telemetrycontrollers.MetricPipelineControllerConfig{
			Config: metricpipeline.Config{
				Agent: otelcollector.AgentConfig{
					Config: otelcollector.Config{
						Namespace: telemetryNamespace,
						BaseName:  "telemetry-metric-agent",
					},
					DaemonSet: otelcollector.DaemonSetConfig{
						Image:             metricGatewayImage,
						PriorityClassName: metricGatewayPriorityClass,
						CPULimit:          resource.MustParse("1"),
						MemoryLimit:       resource.MustParse("1200Mi"),
						CPURequest:        resource.MustParse("15m"),
						MemoryRequest:     resource.MustParse("50Mi"),
					},
				},
				Gateway: otelcollector.GatewayConfig{
					Config: otelcollector.Config{
						Namespace: telemetryNamespace,
						BaseName:  "telemetry-metric-gateway",
					},
					Deployment: otelcollector.DeploymentConfig{
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
					OTLPServiceName: metricOTLPServiceName,
				},
				MaxPipelines:  maxMetricPipelines,
				ModuleVersion: version,
			},
			TelemetryNamespace: telemetryNamespace,
			SelfMonitorName:    selfMonitorName,
		},
	)
	if err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "MetricPipeline")
		os.Exit(1)
	}
	if err := metricPipelineController.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to setup controller", "controller", "MetricPipeline")
		os.Exit(1)
	}
}

func enableWebhookServer(mgr manager.Manager, webhookConfig telemetry.WebhookConfig) {
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

func createLogPipelineValidator(client client.Client) *logpipelinewebhook.ValidatingWebhookHandler {
	return logpipelinewebhook.NewValidatingWebhookHandler(
		client,
		validation.NewVariablesValidator(client),
		validation.NewMaxPipelinesValidator(maxLogPipelines),
		validation.NewFilesValidator(),
		admission.NewDecoder(scheme),
		dryrun.NewDryRunner(client, createDryRunConfig()),
		&telemetryv1alpha1.LogPipelineValidationConfig{DeniedOutPutPlugins: parsePlugins(fluentBitDeniedOutputPlugins), DeniedFilterPlugins: parsePlugins(fluentBitDeniedFilterPlugins)})
}

func createLogParserValidator(client client.Client) *logparserwebhook.ValidatingWebhookHandler {
	return logparserwebhook.NewValidatingWebhookHandler(
		client,
		dryrun.NewDryRunner(client, createDryRunConfig()),
		admission.NewDecoder(scheme))
}

func createSelfMonitoringConfig() telemetry.SelfMonitorConfig {
	return telemetry.SelfMonitorConfig{
		Config: selfmonitor.Config{
			BaseName:  selfMonitorName,
			Namespace: telemetryNamespace,
			Deployment: selfmonitor.DeploymentConfig{
				Image:             selfMonitorImage,
				PriorityClassName: selfMonitorPriorityClass,
			},
		},
		WebhookScheme: "https",
		WebhookURL:    fmt.Sprintf("%s.%s.svc", webhookServiceName, telemetryNamespace),
	}
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
