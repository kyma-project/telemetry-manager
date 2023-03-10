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
	"errors"
	"flag"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetrycontrollers "github.com/kyma-project/telemetry-manager/controllers/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	"github.com/kyma-project/telemetry-manager/internal/kubernetes"
	"github.com/kyma-project/telemetry-manager/internal/logger"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	logparserreconciler "github.com/kyma-project/telemetry-manager/internal/reconciler/logparser"
	logpipelinereconciler "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline"
	tracepipelinereconciler "github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline"
	logpipelineresources "github.com/kyma-project/telemetry-manager/internal/resources/logpipeline"
	"github.com/kyma-project/telemetry-manager/internal/setup"
	"github.com/kyma-project/telemetry-manager/webhook/dryrun"
	logparserwebhook "github.com/kyma-project/telemetry-manager/webhook/logparser"
	logparservalidation "github.com/kyma-project/telemetry-manager/webhook/logparser/validation"
	logpipelinewebhook "github.com/kyma-project/telemetry-manager/webhook/logpipeline"
	logpipelinevalidation "github.com/kyma-project/telemetry-manager/webhook/logpipeline/validation"
	//nolint:gosec
	_ "net/http/pprof"

	"github.com/go-logr/zapr"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sWebhook "sigs.k8s.io/controller-runtime/pkg/webhook"
	//+kubebuilder:scaffold:imports
)

var (
	certDir                string
	deniedFilterPlugins    string
	deniedOutputPlugins    string
	enableLogging          bool
	enableTracing          bool
	logLevel               string
	scheme                 = runtime.NewScheme()
	setupLog               = ctrl.Log.WithName("setup")
	dynamicLoglevel        = zap.NewAtomicLevel()
	configureLogLevelOnFly *logger.LogLevel

	traceCollectorImage         string
	traceCollectorPriorityClass string
	traceCollectorCPULimit      string
	traceCollectorMemoryLimit   string
	traceCollectorCPURequest    string
	traceCollectorMemoryRequest string

	fluentBitMemoryBufferLimit         string
	fluentBitFsBufferLimit             string
	fluentBitCPULimit                  string
	fluentBitMemoryLimit               string
	fluentBitCPURequest                string
	fluentBitMemoryRequest             string
	maxLogPipelines                    int
	fluentBitImageVersion              string
	fluentBitExporterVersion           string
	fluentBitConfigPrepperImageVersion string
	fluentBitPriorityClassName         string

	enableWebhook bool
)

const (
	otelImage              = "eu.gcr.io/kyma-project/tpi/otel-collector:0.72.0-734399a6"
	overrideConfigMapName  = "telemetry-override-config"
	fluentBitImage         = "eu.gcr.io/kyma-project/tpi/fluent-bit:2.0.9-f89e8b78"
	fluentBitExporterImage = "eu.gcr.io/kyma-project/directory-size-exporter:v20221020-e314a071"

	telemetryNamespace = "kyma-system"

	fluentBitDaemonSet = "telemetry-fluent-bit"
	webhookServiceName = "telemetry-operator-webhook"
)

//nolint:gochecknoinits
func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(telemetryv1alpha1.AddToScheme(scheme))
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

//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;
//+kubebuilder:rbac:groups="",namespace=kyma-system,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
//+kubebuilder:rbac:groups="",namespace=kyma-system,resources=services,verbs=get;list;watch;create;update;patch;delete

//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",namespace=kyma-system,resources=secrets,verbs=create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch
//+kubebuilder:rbac:groups="",namespace=kyma-system,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;

//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;
//+kubebuilder:rbac:groups=apps,namespace=kyma-system,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,namespace=kyma-system,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch

//+kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=create;get;update;

//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete

func main() {
	flag.BoolVar(&enableLogging, "enable-logging", true, "Enable configurable logging.")
	flag.BoolVar(&enableTracing, "enable-tracing", true, "Enable configurable tracing.")
	flag.StringVar(&logLevel, "log-level", getEnvOrDefault("APP_LOG_LEVEL", "debug"), "Log level (debug, info, warn, error, fatal)")
	flag.StringVar(&certDir, "cert-dir", ".", "Webhook TLS certificate directory")

	flag.StringVar(&traceCollectorImage, "trace-collector-image", otelImage, "Image for tracing OpenTelemetry Collector")
	flag.StringVar(&traceCollectorPriorityClass, "trace-collector-priority-class", "", "Priority class name for tracing OpenTelemetry Collector")
	flag.StringVar(&traceCollectorCPULimit, "trace-collector-cpu-limit", "1", "CPU limit for tracing OpenTelemetry Collector")
	flag.StringVar(&traceCollectorMemoryLimit, "trace-collector-memory-limit", "1Gi", "Memory limit for tracing OpenTelemetry Collector")
	flag.StringVar(&traceCollectorCPURequest, "trace-collector-cpu-request", "25m", "CPU request for tracing OpenTelemetry Collector")
	flag.StringVar(&traceCollectorMemoryRequest, "trace-collector-memory-request", "32Mi", "Memory request for tracing OpenTelemetry Collector")

	flag.StringVar(&fluentBitMemoryBufferLimit, "fluent-bit-memory-buffer-limit", "10M", "Fluent Bit memory buffer limit per log pipeline")
	flag.StringVar(&fluentBitFsBufferLimit, "fluent-bit-filesystem-buffer-limit", "1G", "Fluent Bit filesystem buffer limit per log pipeline")
	flag.StringVar(&deniedFilterPlugins, "fluent-bit-denied-filter-plugins", "", "Comma separated list of denied filter plugins even if allowUnsupportedPlugins is enabled. If empty, all filter plugins are allowed.")
	flag.StringVar(&fluentBitCPULimit, "fluent-bit-cpu-limit", "1", "CPU limit for tracing fluent-bit")
	flag.StringVar(&fluentBitMemoryLimit, "fluent-bit-memory-limit", "1Gi", "Memory limit for fluent-bit")
	flag.StringVar(&fluentBitCPURequest, "fluent-bit-cpu-request", "400m", "CPU request for fluent-bit")
	flag.StringVar(&fluentBitMemoryRequest, "fluent-bit-memory-request", "256Mi", "Memory request for fluent-bit")
	flag.StringVar(&fluentBitImageVersion, "fluent-bit-image", fluentBitImage, "Image for fluent-bit")
	flag.StringVar(&fluentBitExporterVersion, "fluent-bit-exporter-image", fluentBitExporterImage, "Image for exporting fluent bit filesystem usage")
	flag.StringVar(&fluentBitPriorityClassName, "fluent-bit-priority-class-name", "", "Name of the priority class of fluent bit ")

	flag.StringVar(&deniedOutputPlugins, "fluent-bit-denied-output-plugins", "", "Comma separated list of denied output plugins even if allowUnsupportedPlugins is enabled. If empty, all output plugins are allowed.")
	flag.IntVar(&maxLogPipelines, "fluent-bit-max-pipelines", 5, "Maximum number of LogPipelines to be created. If 0, no limit is applied.")

	flag.BoolVar(&enableWebhook, "validating-webhook-enabled", false, "Create validating webhook for LogPipelines and LogParsers.")

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

		err = server.ListenAndServe()
		if err != nil {
			setupLog.Error(err, "Cannot start pprof server")
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

	certificate, key, err := setup.GenerateCert(webhookServiceName, telemetryNamespace)
	if err != nil {
		setupLog.Error(err, "failed to generate certificate")
		os.Exit(1)
	}

	err = os.WriteFile(path.Join(certDir, "tls.crt"), certificate, 0600)
	if err != nil {
		setupLog.Error(err, "failed to write tls.crt")
		os.Exit(1)
	}

	err = os.WriteFile(path.Join(certDir, "tls.key"), key, 0600)
	if err != nil {
		setupLog.Error(err, "failed to write tls.key")
		os.Exit(1)
	}
	syncPeriod := 1 * time.Hour

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		SyncPeriod:              &syncPeriod,
		Scheme:                  scheme,
		MetricsBindAddress:      ":8080",
		Port:                    9443,
		HealthProbeBindAddress:  ":8081",
		LeaderElection:          true,
		LeaderElectionNamespace: telemetryNamespace,
		LeaderElectionID:        "cdd7ef0b.kyma-project.io",
		CertDir:                 certDir,
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

		webhookService := types.NamespacedName{
			Name:      webhookServiceName,
			Namespace: telemetryNamespace,
		}

		if err = setup.EnsureValidatingWebhookConfig(k8sClient, webhookService, certificate); err != nil {
			setupLog.Error(err, "Failed to patch ValidatingWebhookConfigurations")
			os.Exit(1)
		}
		setupLog.Info("Updated ValidatingWebhookConfiguration")
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Failed to run manager")
		os.Exit(1)
	}
}

func validateFlags() error {

	if logLevel != "debug" && logLevel != "info" && logLevel != "warn" && logLevel != "error" && logLevel != "fatal" {
		return errors.New("--log-level has to be one of debug, info, warn, error, fatal")
	}
	return nil
}

func createLogPipelineReconciler(client client.Client) *telemetrycontrollers.LogPipelineReconciler {
	config := logpipelinereconciler.Config{
		SectionsConfigMap: types.NamespacedName{Name: "telemetry-fluent-bit-sections", Namespace: telemetryNamespace},
		FilesConfigMap:    types.NamespacedName{Name: "telemetry-fluent-bit-files", Namespace: telemetryNamespace},
		EnvSecret:         types.NamespacedName{Name: "telemetry-fluent-bit-env", Namespace: telemetryNamespace},
		DaemonSet:         types.NamespacedName{Name: fluentBitDaemonSet, Namespace: telemetryNamespace},
		OverrideConfigMap: types.NamespacedName{Name: overrideConfigMapName, Namespace: telemetryNamespace},
		PipelineDefaults:  createPipelineDefaults(),
		DaemonSetConfig: logpipelineresources.DaemonSetConfig{
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
	overrides := overrides.New(configureLogLevelOnFly, &kubernetes.ConfigmapProber{Client: client})

	return telemetrycontrollers.NewLogPipelineReconciler(
		client,
		logpipelinereconciler.NewReconciler(client, config, &kubernetes.DaemonSetProber{Client: client}, overrides),
		config)
}

func createLogParserReconciler(client client.Client) *telemetrycontrollers.LogParserReconciler {
	config := logparserreconciler.Config{
		ParsersConfigMap: types.NamespacedName{Name: "telemetry-fluent-bit-parsers", Namespace: telemetryNamespace},
		DaemonSet:        types.NamespacedName{Name: fluentBitDaemonSet, Namespace: telemetryNamespace},
	}
	overrides := overrides.New(configureLogLevelOnFly, &kubernetes.ConfigmapProber{Client: client})

	return telemetrycontrollers.NewLogParserReconciler(
		client,
		logparserreconciler.NewReconciler(
			client,
			config,
			&kubernetes.DaemonSetProber{Client: client},
			&kubernetes.DaemonSetAnnotator{Client: client},
			overrides,
		),
		config,
	)
}

func createLogPipelineValidator(client client.Client) *logpipelinewebhook.ValidatingWebhookHandler {
	return logpipelinewebhook.NewValidatingWebhookHandler(
		client,
		logpipelinevalidation.NewInputValidator(),
		logpipelinevalidation.NewVariablesValidator(client),
		logpipelinevalidation.NewFilterValidator(parsePlugins(deniedFilterPlugins)...),
		logpipelinevalidation.NewMaxPipelinesValidator(maxLogPipelines),
		logpipelinevalidation.NewOutputValidator(parsePlugins(deniedOutputPlugins)...),
		logpipelinevalidation.NewFilesValidator(),
		dryrun.NewDryRunner(client, createDryRunConfig()))
}

func createLogParserValidator(client client.Client) *logparserwebhook.ValidatingWebhookHandler {
	return logparserwebhook.NewValidatingWebhookHandler(
		client,
		logparservalidation.NewParserValidator(),
		dryrun.NewDryRunner(client, createDryRunConfig()))
}

func createTracePipelineReconciler(client client.Client) *telemetrycontrollers.TracePipelineReconciler {
	config := tracepipelinereconciler.Config{
		Namespace: telemetryNamespace,
		Deployment: tracepipelinereconciler.DeploymentConfig{
			Image:             traceCollectorImage,
			PriorityClassName: traceCollectorPriorityClass,
			CPULimit:          resource.MustParse(traceCollectorCPULimit),
			MemoryLimit:       resource.MustParse(traceCollectorMemoryLimit),
			CPURequest:        resource.MustParse(traceCollectorCPURequest),
			MemoryRequest:     resource.MustParse(traceCollectorMemoryRequest),
		},

		OverrideConfigMap: types.NamespacedName{Name: overrideConfigMapName, Namespace: telemetryNamespace},
	}
	overrides := overrides.New(configureLogLevelOnFly, &kubernetes.ConfigmapProber{Client: client})

	return telemetrycontrollers.NewTracePipelineReconciler(
		client,
		tracepipelinereconciler.NewReconciler(client, config, &kubernetes.DeploymentProber{Client: client}, overrides),
	)
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
