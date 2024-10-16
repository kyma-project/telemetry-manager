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
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/zapr"
	"go.uber.org/zap/zapcore"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/controllers/operator"
	telemetrycontrollers "github.com/kyma-project/telemetry-manager/controllers/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/logger"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/resources/selfmonitor"
	selfmonitorwebhook "github.com/kyma-project/telemetry-manager/internal/selfmonitor/webhook"
	"github.com/kyma-project/telemetry-manager/internal/webhookcert"
	logparserwebhook "github.com/kyma-project/telemetry-manager/webhook/logparser"
	logpipelinewebhook "github.com/kyma-project/telemetry-manager/webhook/logpipeline"
	"github.com/kyma-project/telemetry-manager/webhook/logpipeline/validation"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup").WithValues("version", version)
	//TODO: replace with build version based on git revision
	version = "main"

	// Operator flags
	certDir                   string
	enableV1Beta1LogPipelines bool
	fluentBitExporterImage    string
	fluentBitImage            string
	highPriorityClassName     string
	normalPriorityClassName   string
	otelCollectorImage        string
	selfMonitorImage          string
	telemetryNamespace        = "default"
)

const (
	defaultFluentBitExporterImage = "europe-docker.pkg.dev/kyma-project/prod/directory-size-exporter:v20241001-21f80ba0"
	defaultFluentBitImage         = "europe-docker.pkg.dev/kyma-project/prod/external/fluent/fluent-bit:3.1.9"
	defaultOTelCollectorImage     = "europe-docker.pkg.dev/kyma-project/prod/kyma-otel-collector:0.111.0-main"
	defaultSelfMonitorImage       = "europe-docker.pkg.dev/kyma-project/prod/tpi/telemetry-self-monitor:2.53.2-cc4f64c"
)

const (
	telemetryNamespaceEnvVar = "MANAGER_NAMESPACE"
	metricOTLPServiceName    = "telemetry-otlp-metrics"
	selfMonitorName          = "telemetry-self-monitor"
	traceOTLPServiceName     = "telemetry-otlp-traces"
	webhookServerPort        = 9443
	webhookServiceName       = "telemetry-manager-webhook"
)

//nolint:gochecknoinits // Runtime's scheme addition is required.
func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))

	utilruntime.Must(telemetryv1alpha1.AddToScheme(scheme))
	utilruntime.Must(operatorv1alpha1.AddToScheme(scheme))
	utilruntime.Must(istiosecurityclientv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// +kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=logpipelines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=logpipelines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=logpipelines/finalizers,verbs=update
// +kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=logparsers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=logparsers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=logparsers/finalizers,verbs=update
// +kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=tracepipelines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=tracepipelines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=metricpipelines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=metricpipelines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=metricpipelines/finalizers,verbs=update

// +kubebuilder:rbac:groups=operator.kyma-project.io,namespace=system,resources=telemetries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.kyma-project.io,namespace=system,resources=telemetries/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.kyma-project.io,namespace=system,resources=telemetries/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=telemetries,verbs=get;list;watch

// +kubebuilder:rbac:groups="",namespace=system,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",namespace=system,resources=services,verbs=get;list;watch;create;update;patch;delete

// +kubebuilder:rbac:groups="",namespace=system,resources=secrets,verbs=create;update;patch;delete
// +kubebuilder:rbac:groups="",namespace=system,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes/metrics,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes/stats,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes/proxy,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes/spec,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=replicationcontrollers,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=replicationcontrollers/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch
// +kubebuilder:rbac:urls=/metrics,verbs=get
// +kubebuilder:rbac:urls=/metrics/cadvisor,verbs=get

// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;patch

// +kubebuilder:rbac:groups=apps,namespace=system,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,namespace=system,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch

// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;create;update;patch;delete

// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete

// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.k8s.io,namespace=system,resources=networkpolicies,verbs=create;update;patch;delete

// +kubebuilder:rbac:groups=security.istio.io,resources=peerauthentications,verbs=get;list;watch
// +kubebuilder:rbac:groups=security.istio.io,namespace=system,resources=peerauthentications,verbs=create;update;patch;delete

// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch

// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch

// +kubebuilder:rbac:groups=extensions,resources=daemonsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=extensions,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=extensions,resources=replicasets,verbs=get;list;watch

// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func main() {
	if err := run(); err != nil {
		setupLog.Error(err, "Manager exited with error")
		os.Exit(1)
	}
}

func run() error {
	flag.BoolVar(&enableV1Beta1LogPipelines, "enable-v1beta1-log-pipelines", false, "Enable v1beta1 log pipelines CRD")
	flag.StringVar(&certDir, "cert-dir", ".", "Webhook TLS certificate directory")
	flag.StringVar(&fluentBitExporterImage, "fluent-bit-exporter-image", defaultFluentBitExporterImage, "Image for exporting fluent bit filesystem usage")
	flag.StringVar(&fluentBitImage, "fluent-bit-image", defaultFluentBitImage, "Image for fluent-bit")
	flag.StringVar(&highPriorityClassName, "high-priority-class-name", "", "High priority class name used by managed DaemonSets")
	flag.StringVar(&normalPriorityClassName, "normal-priority-class-name", "", "Normal priority class name used by managed Deployments")
	flag.StringVar(&otelCollectorImage, "otel-collector-image", defaultOTelCollectorImage, "Image for OpenTelemetry Collector")
	flag.StringVar(&selfMonitorImage, "self-monitor-image", defaultSelfMonitorImage, "Image for self-monitor")

	flag.Parse()

	telemetryNamespace, _ = os.LookupEnv(telemetryNamespaceEnvVar)

	overrides.AtomicLevel().SetLevel(zapcore.InfoLevel)

	zapLogger, err := logger.New(overrides.AtomicLevel())
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}

	defer zapLogger.Sync() //nolint:errcheck // if flusing logs fails there is nothing else	we can do

	ctrl.SetLogger(zapr.NewLogger(zapLogger))

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
			Port:    webhookServerPort,
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
		return fmt.Errorf("failed to start manager: %w", err)
	}

	tracePipelineReconcileTriggerChan := make(chan event.GenericEvent)
	if err := setupTracePipelineController(mgr, tracePipelineReconcileTriggerChan); err != nil {
		return fmt.Errorf("failed to enable trace pipeline controller: %w", err)
	}

	metricPipelineReconcileTriggerChan := make(chan event.GenericEvent)
	if err := setupMetricPipelineController(mgr, metricPipelineReconcileTriggerChan); err != nil {
		return fmt.Errorf("failed to enable metric pipeline controller: %w", err)
	}

	logPipelineReconcileTriggerChan := make(chan event.GenericEvent)
	if err := setupLogPipelineController(mgr, logPipelineReconcileTriggerChan); err != nil {
		return fmt.Errorf("failed to enable log pipeline controller: %w", err)
	}

	webhookConfig := createWebhookConfig()
	selfMonitorConfig := createSelfMonitoringConfig()

	if err := enableTelemetryModuleController(mgr, webhookConfig, selfMonitorConfig); err != nil {
		return fmt.Errorf("failed to enable telemetry module controller: %w", err)
	}

	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", mgr.GetWebhookServer().StartedChecker()); err != nil {
		return fmt.Errorf("failed to add health check: %w", err)
	}

	if err := mgr.AddReadyzCheck("readyz", mgr.GetWebhookServer().StartedChecker()); err != nil {
		return fmt.Errorf("failed to add ready check: %w", err)
	}

	if err := ensureWebhookCert(mgr, webhookConfig); err != nil {
		return fmt.Errorf("failed to enable webhook server: %w", err)
	}

	mgr.GetWebhookServer().Register("/validate-logpipeline", &webhook.Admission{
		Handler: createLogPipelineValidator(mgr.GetClient()),
	})
	mgr.GetWebhookServer().Register("/validate-logparser", &webhook.Admission{
		Handler: createLogParserValidator(mgr.GetClient()),
	})
	mgr.GetWebhookServer().Register("/api/v2/alerts", selfmonitorwebhook.NewHandler(
		mgr.GetClient(),
		selfmonitorwebhook.WithTracePipelineSubscriber(tracePipelineReconcileTriggerChan),
		selfmonitorwebhook.WithMetricPipelineSubscriber(metricPipelineReconcileTriggerChan),
		selfmonitorwebhook.WithLogPipelineSubscriber(logPipelineReconcileTriggerChan),
		selfmonitorwebhook.WithLogger(ctrl.Log.WithName("self-monitor-webhook"))))

	setupLog.Info("Starting manager", "version", version)

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		return fmt.Errorf("failed to start manager: %w", err)
	}

	return nil
}

func enableTelemetryModuleController(mgr manager.Manager, webhookConfig telemetry.WebhookConfig, selfMonitorConfig telemetry.SelfMonitorConfig) error {
	setupLog.Info("Setting up telemetry controller")

	telemetryController := operator.NewTelemetryController(
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

	if err := telemetryController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup telemetry controller: %w", err)
	}

	return nil
}

func setupLogPipelineController(mgr manager.Manager, reconcileTriggerChan <-chan event.GenericEvent) error {
	if enableV1Beta1LogPipelines {
		setupLog.Info("Registering conversion webhooks for LogPipelines")
		utilruntime.Must(telemetryv1beta1.AddToScheme(scheme))
		// Register conversion webhooks for LogPipelines
		if err := ctrl.NewWebhookManagedBy(mgr).
			For(&telemetryv1alpha1.LogPipeline{}).
			Complete(); err != nil {
			return fmt.Errorf("failed to create v1alpha1 conversion webhook: %w", err)
		}

		if err := ctrl.NewWebhookManagedBy(mgr).
			For(&telemetryv1beta1.LogPipeline{}).
			Complete(); err != nil {
			return fmt.Errorf("failed to create v1beta1 conversion webhook: %w", err)
		}
	}

	setupLog.Info("Setting up logpipeline controller")

	logPipelineController, err := telemetrycontrollers.NewLogPipelineController(
		mgr.GetClient(),
		reconcileTriggerChan,
		telemetrycontrollers.LogPipelineControllerConfig{
			ExporterImage:      fluentBitExporterImage,
			FluentBitImage:     fluentBitImage,
			PriorityClassName:  highPriorityClassName,
			RestConfig:         mgr.GetConfig(),
			SelfMonitorName:    selfMonitorName,
			TelemetryNamespace: telemetryNamespace,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create logpipeline controller: %w", err)
	}

	if err := logPipelineController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup logpipeline controller: %w", err)
	}

	setupLog.Info("Setting up logparser controller")

	logParserController := telemetrycontrollers.NewLogParserController(
		mgr.GetClient(),
		telemetrycontrollers.LogParserControllerConfig{
			TelemetryNamespace: telemetryNamespace,
		},
	)

	if err := logParserController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup logparser controller: %w", err)
	}

	return nil
}

func setupTracePipelineController(mgr manager.Manager, reconcileTriggerChan <-chan event.GenericEvent) error {
	setupLog.Info("Setting up tracepipeline controller")

	tracePipelineController, err := telemetrycontrollers.NewTracePipelineController(
		mgr.GetClient(),
		reconcileTriggerChan,
		telemetrycontrollers.TracePipelineControllerConfig{
			RestConfig:                    mgr.GetConfig(),
			OTelCollectorImage:            otelCollectorImage,
			SelfMonitorName:               selfMonitorName,
			TelemetryNamespace:            telemetryNamespace,
			TraceGatewayPriorityClassName: normalPriorityClassName,
			TraceGatewayServiceName:       traceOTLPServiceName,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create tracepipeline controller: %w", err)
	}

	if err := tracePipelineController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup tracepipeline controller: %w", err)
	}

	return nil
}

func setupMetricPipelineController(mgr manager.Manager, reconcileTriggerChan <-chan event.GenericEvent) error {
	setupLog.Info("Setting up metricpipeline controller")

	metricPipelineController, err := telemetrycontrollers.NewMetricPipelineController(
		mgr.GetClient(),
		reconcileTriggerChan,
		telemetrycontrollers.MetricPipelineControllerConfig{
			MetricAgentPriorityClassName:   highPriorityClassName,
			MetricGatewayPriorityClassName: normalPriorityClassName,
			MetricGatewayServiceName:       metricOTLPServiceName,
			ModuleVersion:                  version,
			OTelCollectorImage:             otelCollectorImage,
			RestConfig:                     mgr.GetConfig(),
			SelfMonitorName:                selfMonitorName,
			TelemetryNamespace:             telemetryNamespace,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create metricpipeline controller: %w", err)
	}

	if err := metricPipelineController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup metricpipeline controller: %w", err)
	}

	return nil
}

func ensureWebhookCert(mgr manager.Manager, webhookConfig telemetry.WebhookConfig) error {
	// Create own client since manager might not be started while using
	clientOptions := client.Options{
		Scheme: scheme,
	}

	k8sClient, err := client.New(mgr.GetConfig(), clientOptions)
	if err != nil {
		return fmt.Errorf("failed to create webhook client: %w", err)
	}

	if err = webhookcert.EnsureCertificate(context.Background(), k8sClient, webhookConfig.CertConfig); err != nil {
		return fmt.Errorf("failed to ensure webhook cert: %w", err)
	}

	setupLog.Info("Ensured webhook cert")

	return nil
}

func setNamespaceFieldSelector() fields.Selector {
	return fields.SelectorFromSet(fields.Set{"metadata.namespace": telemetryNamespace})
}

func createLogPipelineValidator(client client.Client) *logpipelinewebhook.ValidatingWebhookHandler {
	// TODO: Align max log pipeline enforcement with the method used in the TracePipeline/MetricPipeline controllers,
	// replacing the current validating webhook approach.
	const maxLogPipelines = 5

	return logpipelinewebhook.NewValidatingWebhookHandler(
		client,
		validation.NewVariablesValidator(client),
		validation.NewMaxPipelinesValidator(maxLogPipelines),
		validation.NewFilesValidator(),
		admission.NewDecoder(scheme),
	)
}

func createLogParserValidator(client client.Client) *logparserwebhook.ValidatingWebhookHandler {
	return logparserwebhook.NewValidatingWebhookHandler(
		client,
		admission.NewDecoder(scheme))
}

func createSelfMonitoringConfig() telemetry.SelfMonitorConfig {
	return telemetry.SelfMonitorConfig{
		Config: selfmonitor.Config{
			BaseName:  selfMonitorName,
			Namespace: telemetryNamespace,
			Deployment: selfmonitor.DeploymentConfig{
				Image:             selfMonitorImage,
				PriorityClassName: normalPriorityClassName,
			},
		},
		WebhookScheme: "https",
		WebhookURL:    fmt.Sprintf("%s.%s.svc", webhookServiceName, telemetryNamespace),
	}
}

func createWebhookConfig() telemetry.WebhookConfig {
	return telemetry.WebhookConfig{
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
