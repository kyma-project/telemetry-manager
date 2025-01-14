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
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/controllers/operator"
	telemetrycontrollers "github.com/kyma-project/telemetry-manager/controllers/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/featureflags"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/resources/selfmonitor"
	selfmonitorwebhook "github.com/kyma-project/telemetry-manager/internal/selfmonitor/webhook"
	loggerutils "github.com/kyma-project/telemetry-manager/internal/utils/logger"
	"github.com/kyma-project/telemetry-manager/internal/webhookcert"
	logparserwebhookv1alpha1 "github.com/kyma-project/telemetry-manager/webhook/logparser/v1alpha1"
	logpipelinewebhookv1alpha1 "github.com/kyma-project/telemetry-manager/webhook/logpipeline/v1alpha1"
	logpipelinewebhookv1beta1 "github.com/kyma-project/telemetry-manager/webhook/logpipeline/v1beta1"
	metricpipelinewebhookv1alpha1 "github.com/kyma-project/telemetry-manager/webhook/metricpipeline/v1alpha1"
	metricpipelinewebhookv1beta1 "github.com/kyma-project/telemetry-manager/webhook/metricpipeline/v1beta1"
	tracepipelinewebhookv1alpha1 "github.com/kyma-project/telemetry-manager/webhook/tracepipeline/v1alpha1"
	tracepipelinewebhookv1beta1 "github.com/kyma-project/telemetry-manager/webhook/tracepipeline/v1beta1"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	scheme             = runtime.NewScheme()
	setupLog           = ctrl.Log.WithName("setup")
	telemetryNamespace string
	// TODO: replace with build version based on git revision
	version = "main"

	// Operator flags
	certDir                   string
	enableV1Beta1LogPipelines bool
	enableLogPipelinesOTLP    bool

	highPriorityClassName   string
	normalPriorityClassName string

	fluentBitExporterImage string
	fluentBitImage         string
	otelCollectorImage     string
	selfMonitorImage       string
)

const (
	defaultFluentBitExporterImage = "europe-docker.pkg.dev/kyma-project/prod/directory-size-exporter:v20241212-e4adf27f"
	defaultFluentBitImage         = "europe-docker.pkg.dev/kyma-project/prod/external/fluent/fluent-bit:3.2.2"
	defaultOTelCollectorImage     = "europe-docker.pkg.dev/kyma-project/prod/kyma-otel-collector:0.115.0-main"
	defaultSelfMonitorImage       = "europe-docker.pkg.dev/kyma-project/prod/tpi/telemetry-self-monitor:3.0.1-857b814"

	cacheSyncPeriod           = 1 * time.Minute
	telemetryNamespaceEnvVar  = "MANAGER_NAMESPACE"
	telemetryNamespaceDefault = "default"
	selfMonitorName           = "telemetry-self-monitor"
	webhookServiceName        = "telemetry-manager-webhook"

	healthProbePort = 8081
	metricsPort     = 8080
	pprofPort       = 6060
	webhookPort     = 9443
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

func main() {
	if err := run(); err != nil {
		setupLog.Error(err, "Manager exited with error")
		os.Exit(1)
	}
}

func run() error {
	flag.BoolVar(&enableV1Beta1LogPipelines, "enable-v1beta1-log-pipelines", false, "Enable v1beta1 log pipelines CRD")
	flag.BoolVar(&enableLogPipelinesOTLP, "enable-log-pipelines-otlp", false, "Enable otlp input and output for log pipelines")
	flag.StringVar(&certDir, "cert-dir", ".", "Webhook TLS certificate directory")

	flag.StringVar(&highPriorityClassName, "high-priority-class-name", "", "High priority class name used by managed DaemonSets")
	flag.StringVar(&normalPriorityClassName, "normal-priority-class-name", "", "Normal priority class name used by managed Deployments")

	flag.StringVar(&fluentBitExporterImage, "fluent-bit-exporter-image", defaultFluentBitExporterImage, "Image for exporting fluent bit filesystem usage")
	flag.StringVar(&fluentBitImage, "fluent-bit-image", defaultFluentBitImage, "Image for fluent-bit")
	flag.StringVar(&otelCollectorImage, "otel-collector-image", defaultOTelCollectorImage, "Image for OpenTelemetry Collector")
	flag.StringVar(&selfMonitorImage, "self-monitor-image", defaultSelfMonitorImage, "Image for self-monitor")

	flag.Parse()

	featureflags.Set(featureflags.V1Beta1, enableV1Beta1LogPipelines)
	featureflags.Set(featureflags.LogPipelineOTLP, enableLogPipelinesOTLP)

	telemetryNamespace = os.Getenv(telemetryNamespaceEnvVar)
	if telemetryNamespace == "" {
		telemetryNamespace = telemetryNamespaceDefault
	}

	overrides.AtomicLevel().SetLevel(zapcore.InfoLevel)

	zapLogger, err := loggerutils.New(overrides.AtomicLevel())
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}

	defer zapLogger.Sync() //nolint:errcheck // if flusing logs fails there is nothing else	we can do

	ctrl.SetLogger(zapr.NewLogger(zapLogger))

	setupLog.Info("Starting Telemetry Manager", "version", version)

	for _, flag := range featureflags.EnabledFlags() {
		setupLog.Info("Enabled feature flag", "flag", flag)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		Metrics:                 metricsserver.Options{BindAddress: fmt.Sprintf(":%d", metricsPort)},
		HealthProbeBindAddress:  fmt.Sprintf(":%d", healthProbePort),
		PprofBindAddress:        fmt.Sprintf(":%d", pprofPort),
		LeaderElection:          true,
		LeaderElectionNamespace: telemetryNamespace,
		LeaderElectionID:        "cdd7ef0b.kyma-project.io",
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    webhookPort,
			CertDir: certDir,
		}),
		Cache: cache.Options{
			SyncPeriod: ptr.To(cacheSyncPeriod),

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

	if err := setupAdmissionsWebhooks(mgr); err != nil {
		return fmt.Errorf("failed to setup admission webhooks: %w", err)
	}

	mgr.GetWebhookServer().Register("/api/v2/alerts", selfmonitorwebhook.NewHandler(
		mgr.GetClient(),
		selfmonitorwebhook.WithTracePipelineSubscriber(tracePipelineReconcileTriggerChan),
		selfmonitorwebhook.WithMetricPipelineSubscriber(metricPipelineReconcileTriggerChan),
		selfmonitorwebhook.WithLogPipelineSubscriber(logPipelineReconcileTriggerChan),
		selfmonitorwebhook.WithLogger(ctrl.Log.WithName("self-monitor-webhook"))))

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		return fmt.Errorf("failed to start manager: %w", err)
	}

	return nil
}

func setupAdmissionsWebhooks(mgr manager.Manager) error {
	if err := metricpipelinewebhookv1alpha1.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup metric pipeline v1alpha1 webhook: %w", err)
	}

	if featureflags.IsEnabled(featureflags.V1Beta1) {
		if err := metricpipelinewebhookv1beta1.SetupWithManager(mgr); err != nil {
			return fmt.Errorf("failed to setup metric pipeline v1beta1 webhook: %w", err)
		}
	}

	if err := tracepipelinewebhookv1alpha1.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup trace pipeline v1alpha1 webhook: %w", err)
	}

	if featureflags.IsEnabled(featureflags.V1Beta1) {
		if err := tracepipelinewebhookv1beta1.SetupWithManager(mgr); err != nil {
			return fmt.Errorf("failed to setup trace pipeline v1beta1 webhook: %w", err)
		}
	}

	if err := logpipelinewebhookv1alpha1.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup log pipeline v1alpha1 webhook: %w", err)
	}

	if featureflags.IsEnabled(featureflags.V1Beta1) {
		if err := logpipelinewebhookv1beta1.SetupWithManager(mgr); err != nil {
			return fmt.Errorf("failed to setup log pipeline v1beta1 webhook: %w", err)
		}
	}

	logparserwebhookv1alpha1.SetupWithManager(mgr)

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
					Namespace: telemetryNamespace,
				},
				Metrics: telemetry.MetricsConfig{
					Namespace: telemetryNamespace,
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
	if featureflags.IsEnabled(featureflags.V1Beta1) {
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
			ExporterImage:               fluentBitExporterImage,
			FluentBitImage:              fluentBitImage,
			OTelCollectorImage:          otelCollectorImage,
			FluentBitPriorityClassName:  highPriorityClassName,
			LogGatewayPriorityClassName: normalPriorityClassName,
			RestConfig:                  mgr.GetConfig(),
			SelfMonitorName:             selfMonitorName,
			TelemetryNamespace:          telemetryNamespace,
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
			ValidatingWebhookName: types.NamespacedName{
				Name: "telemetry-validating-webhook.kyma-project.io",
			},
			MutatingWebhookName: types.NamespacedName{
				Name: "telemetry-mutating-webhook.kyma-project.io",
			},
		},
	}
}
