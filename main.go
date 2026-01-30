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
	"log"
	"os"

	"github.com/caarlos0/env/v11"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	istionetworkingclientv1 "istio.io/client-go/pkg/apis/networking/v1"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/discovery"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/controllers/operator"
	telemetrycontrollers "github.com/kyma-project/telemetry-manager/controllers/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/build"
	"github.com/kyma-project/telemetry-manager/internal/cliflags"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/featureflags"
	"github.com/kyma-project/telemetry-manager/internal/istiostatus"
	"github.com/kyma-project/telemetry-manager/internal/metrics"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	selfmonitorwebhook "github.com/kyma-project/telemetry-manager/internal/selfmonitor/webhook"
	loggerutils "github.com/kyma-project/telemetry-manager/internal/utils/logger"
	"github.com/kyma-project/telemetry-manager/internal/webhookcert"
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
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")

	// Operator flags
	certDir                 string
	highPriorityClassName   string
	normalPriorityClassName string
	clusterTrustBundleName  string
	imagePullSecretName     string
	additionalLabels        cliflags.Map
	additionalAnnotations   cliflags.Map
	deployOTLPGateway       bool
)

const (
	webhookServiceName = names.ManagerWebhookService

	healthProbePort = 8081
	metricsPort     = 8080
	pprofPort       = 6060
	webhookPort     = 9443
)

//go:generate bin/envdoc -output docs/config.md -dir . -types=envConfig -files=*.go
type envConfig struct {
	// FluentBitExporterImage is the image used for the Fluent Bit exporter.
	FluentBitExporterImage string `env:"FLUENT_BIT_EXPORTER_IMAGE"`
	// FluentBitImage is the image used for the Fluent Bit log agent.
	FluentBitImage string `env:"FLUENT_BIT_IMAGE"`
	// OTelCollectorImage is the image used all OpenTelemetry Collector based components (metric agent, log agent, metric gateway, log gateway, trace gateway).
	OTelCollectorImage string `env:"OTEL_COLLECTOR_IMAGE"`
	// SelfMonitorImage is the image used for the self-monitoring deployment. This is a customized Prometheus image.
	SelfMonitorImage string `env:"SELF_MONITOR_IMAGE"`
	// AlpineImage is the image used for the chown init containers.
	AlpineImage string `env:"ALPINE_IMAGE"`
	// ImagePullSecret is the name of the image pull secret to use for pulling images of all created workloads (agents, gateways, self-monitor).
	ImagePullSecret string `env:"SKR_IMG_PULL_SECRET" envDefault:""`
	// ManagerNamespace returns the namespace where Telemetry Manager is deployed. In a Kyma setup, this is the same as TargetNamespace.
	ManagerNamespace string `env:"MANAGER_NAMESPACE" envDefault:"default"`
	// TargetNamespace is the namespace where telemetry components should be deployed by Telemetry Manager.
	TargetNamespace string `env:"TARGET_NAMESPACE" envDefault:"default"`
	// OperateInFIPSMode defines whether components should be deployed in FIPS 140-2 compliant way.
	OperateInFIPSMode bool `env:"KYMA_FIPS_MODE_ENABLED" envDefault:"false"`
}

//nolint:gochecknoinits // Runtime's scheme addition is required.
func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))

	utilruntime.Must(telemetryv1alpha1.AddToScheme(scheme))
	utilruntime.Must(operatorv1alpha1.AddToScheme(scheme))
	utilruntime.Must(istiosecurityclientv1.AddToScheme(scheme))
	utilruntime.Must(istionetworkingclientv1.AddToScheme(scheme))
	utilruntime.Must(telemetryv1beta1.AddToScheme(scheme))
	utilruntime.Must(operatorv1beta1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	zapLogger, err := setupSetupLog()
	if err != nil {
		log.Panicf("failed to setup zap logger: %v", err)
	}

	if err := run(); err != nil {
		setupLog.Error(err, "Manager exited with error")
		zapLogger.Sync() //nolint:errcheck // if flushing logs fails there is nothing else	we can do
		os.Exit(1)
	}

	zapLogger.Sync() //nolint:errcheck // if flushing logs fails there is nothing else	we can do
}

func run() error {
	parseFlags()
	initializeFeatureFlags()

	var envCfg envConfig
	if err := env.ParseWithOptions(&envCfg, env.Options{Prefix: "", RequiredIfNoDef: true}); err != nil {
		return fmt.Errorf("failed to parse environment variables: %w", err)
	}

	logBuildAndProcessInfo()

	globals := config.NewGlobal(
		config.WithManagerNamespace(envCfg.ManagerNamespace),
		config.WithTargetNamespace(envCfg.TargetNamespace),
		config.WithOperateInFIPSMode(envCfg.OperateInFIPSMode),
		config.WithVersion(build.GitTag()),
		config.WithImagePullSecretName(imagePullSecretName),
		config.WithClusterTrustBundleName(clusterTrustBundleName),
		config.WithAdditionalLabels(additionalLabels),
		config.WithAdditionalAnnotations(additionalAnnotations),
		config.WithDeployOTLPGateway(featureflags.IsEnabled(featureflags.DeployOTLPGateway)),
	)

	if err := globals.Validate(); err != nil {
		return fmt.Errorf("global configuration validation failed: %w", err)
	}

	setupLog.Info("Global configuration",
		"target_namespace", globals.TargetNamespace(),
		"manager namespace", globals.ManagerNamespace(),
		"version", globals.Version(),
		"fips", globals.OperateInFIPSMode(),
	)

	mgr, err := setupManager(globals)
	if err != nil {
		return err
	}

	err = setupControllersAndWebhooks(mgr, globals, envCfg)
	if err != nil {
		return err
	}

	// +kubebuilder:scaffold:builder

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		return fmt.Errorf("failed to start manager: %w", err)
	}

	return nil
}

func setupSetupLog() (*zap.Logger, error) {
	overrides.AtomicLevel().SetLevel(zapcore.InfoLevel)

	zapLogger, err := loggerutils.New(overrides.AtomicLevel())
	if err != nil {
		return nil, err
	}

	ctrl.SetLogger(zapr.NewLogger(zapLogger))

	return zapLogger, nil
}

func setupControllersAndWebhooks(mgr manager.Manager, globals config.Global, envCfg envConfig) error {
	var (
		tracePipelineReconcileChan  = make(chan event.GenericEvent)
		metricPipelineReconcileChan = make(chan event.GenericEvent)
		logPipelineReconcileChan    = make(chan event.GenericEvent)
	)

	if err := setupTracePipelineController(globals, envCfg, mgr, tracePipelineReconcileChan); err != nil {
		return fmt.Errorf("failed to enable trace pipeline controller: %w", err)
	}

	if err := setupMetricPipelineController(globals, envCfg, mgr, metricPipelineReconcileChan); err != nil {
		return fmt.Errorf("failed to enable metric pipeline controller: %w", err)
	}

	if err := setupLogPipelineController(globals, envCfg, mgr, logPipelineReconcileChan); err != nil {
		return fmt.Errorf("failed to enable log pipeline controller: %w", err)
	}

	webhookCertConfig := createWebhookConfig(globals)

	if err := setupTelemetryController(globals, envCfg, webhookCertConfig, mgr); err != nil {
		return fmt.Errorf("failed to enable telemetry module controller: %w", err)
	}

	if err := mgr.AddHealthzCheck("healthz", mgr.GetWebhookServer().StartedChecker()); err != nil {
		return fmt.Errorf("failed to add health check: %w", err)
	}

	if err := mgr.AddReadyzCheck("readyz", mgr.GetWebhookServer().StartedChecker()); err != nil {
		return fmt.Errorf("failed to add ready check: %w", err)
	}

	if err := ensureWebhookCert(webhookCertConfig, mgr); err != nil {
		return fmt.Errorf("failed to enable webhook server: %w", err)
	}

	if err := setupConversionWebhooks(mgr); err != nil {
		return fmt.Errorf("failed to setup conversion webhooks: %w", err)
	}

	if err := setupAdmissionsWebhooks(mgr); err != nil {
		return fmt.Errorf("failed to setup admission webhooks: %w", err)
	}

	mgr.GetWebhookServer().Register("/api/v2/alerts", selfmonitorwebhook.NewHandler(
		mgr.GetClient(),
		selfmonitorwebhook.WithTracePipelineSubscriber(tracePipelineReconcileChan),
		selfmonitorwebhook.WithMetricPipelineSubscriber(metricPipelineReconcileChan),
		selfmonitorwebhook.WithLogPipelineSubscriber(logPipelineReconcileChan),
		selfmonitorwebhook.WithLogger(ctrl.Log.WithName("self-monitor-webhook"))))

	return nil
}

func setupManager(globals config.Global) (manager.Manager, error) {
	restConfig := ctrl.GetConfigOrDie()

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	isIstioActive := istiostatus.NewChecker(discoveryClient).IsIstioActive(context.Background())

	// The operator handles various resource that are namespace-scoped, and additionally some resources that are cluster-scoped (clusterroles, clusterrolebindings, etc.).
	// For namespace-scoped resources we want to restrict the operator permissions to only fetch resources from a given namespace.
	cacheOptions := map[client.Object]cache.ByObject{
		&appsv1.Deployment{}:          {Field: setNamespaceFieldSelector(globals)},
		&appsv1.ReplicaSet{}:          {Field: setNamespaceFieldSelector(globals)},
		&appsv1.DaemonSet{}:           {Field: setNamespaceFieldSelector(globals)},
		&corev1.ConfigMap{}:           {Namespaces: setConfigMapNamespaceFieldSelector(globals)},
		&corev1.ServiceAccount{}:      {Field: setNamespaceFieldSelector(globals)},
		&corev1.Service{}:             {Field: setNamespaceFieldSelector(globals)},
		&networkingv1.NetworkPolicy{}: {Field: setNamespaceFieldSelector(globals)},
		&corev1.Secret{}:              {Transform: secretCacheTransform},
		&operatorv1beta1.Telemetry{}:  {Field: setNamespaceFieldSelector(globals)},
		&rbacv1.Role{}:                {Field: setNamespaceFieldSelector(globals)},
		&rbacv1.RoleBinding{}:         {Field: setNamespaceFieldSelector(globals)},
	}

	if isIstioActive {
		cacheOptions[&istiosecurityclientv1.PeerAuthentication{}] = cache.ByObject{Field: setNamespaceFieldSelector(globals)}
	}

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                  scheme,
		Metrics:                 metricsserver.Options{BindAddress: fmt.Sprintf(":%d", metricsPort)},
		HealthProbeBindAddress:  fmt.Sprintf(":%d", healthProbePort),
		PprofBindAddress:        fmt.Sprintf(":%d", pprofPort),
		LeaderElection:          true,
		LeaderElectionNamespace: globals.TargetNamespace(),
		LeaderElectionID:        names.ManagerLeaseName,
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    webhookPort,
			CertDir: certDir,
		}),
		Cache: cache.Options{
			ByObject: cacheOptions,
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
		return nil, fmt.Errorf("failed to setup manager: %w", err)
	}

	return mgr, nil
}

func logBuildAndProcessInfo() {
	metrics.BuildInfo.Set(1)

	setupLog.Info("Starting Telemetry Manager", "Build info:", build.InfoMap())

	for _, flg := range featureflags.EnabledFlags() {
		metrics.FeatureFlagsInfo.WithLabelValues(flg.String()).Set(1)
		setupLog.Info("Enabled feature flag", "flag", flg)
	}
}

func initializeFeatureFlags() {
	// Placeholder for future feature flag initializations.
	featureflags.Set(featureflags.DeployOTLPGateway, deployOTLPGateway)
}

func parseFlags() {
	flag.StringVar(&certDir, "cert-dir", ".", "Webhook TLS certificate directory")

	flag.StringVar(&highPriorityClassName, "high-priority-class-name", "", "High priority class name used by managed DaemonSets")
	flag.StringVar(&normalPriorityClassName, "normal-priority-class-name", "", "Normal priority class name used by managed Deployments")
	flag.StringVar(&clusterTrustBundleName, "cluster-trust-bundle-name", "", "The name ClusterTrustBundle resource")
	flag.StringVar(&imagePullSecretName, "image-pull-secret-name", "", "The image pull secret name to use for pulling images of all created workloads (agents, gateways, self-monitor)")
	flag.Var(&additionalLabels, "additional-label", "Additional label to add to all created resources in key=value format")
	flag.Var(&additionalAnnotations, "additional-annotation", "Additional annotation to add to all created resources in key=value format")

	flag.BoolVar(&deployOTLPGateway, "deploy-otlp-gateway", false, "Enable deploying unified OTLP gateway")

	flag.Parse()
}

func setupAdmissionsWebhooks(mgr manager.Manager) error {
	if err := metricpipelinewebhookv1alpha1.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup metric pipeline v1alpha1 webhook: %w", err)
	}

	if err := metricpipelinewebhookv1beta1.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup metric pipeline v1beta1 webhook: %w", err)
	}

	if err := tracepipelinewebhookv1alpha1.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup trace pipeline v1alpha1 webhook: %w", err)
	}

	if err := tracepipelinewebhookv1beta1.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup trace pipeline v1beta1 webhook: %w", err)
	}

	if err := logpipelinewebhookv1alpha1.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup log pipeline v1alpha1 webhook: %w", err)
	}

	if err := logpipelinewebhookv1beta1.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup log pipeline v1beta1 webhook: %w", err)
	}

	return nil
}

func setupTelemetryController(globals config.Global, cfg envConfig, webhookCertConfig webhookcert.Config, mgr manager.Manager) error {
	setupLog.Info("Setting up telemetry controller")

	telemetryController := operator.NewTelemetryController(
		operator.TelemetryControllerConfig{
			Global:                            globals,
			SelfMonitorAlertmanagerWebhookURL: fmt.Sprintf("%s.%s.svc", webhookServiceName, globals.ManagerNamespace()),
			SelfMonitorImage:                  cfg.SelfMonitorImage,
			SelfMonitorPriorityClassName:      normalPriorityClassName,
			WebhookCert:                       webhookCertConfig,
		},
		mgr.GetClient(),
		mgr.GetScheme(),
	)

	if err := telemetryController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup telemetry controller: %w", err)
	}

	return nil
}

func setupLogPipelineController(globals config.Global, cfg envConfig, mgr manager.Manager, reconcileTriggerChan <-chan event.GenericEvent) error {
	setupLog.Info("Setting up logpipeline controller")

	logPipelineController, err := telemetrycontrollers.NewLogPipelineController(
		telemetrycontrollers.LogPipelineControllerConfig{
			Global:                       globals,
			ExporterImage:                cfg.FluentBitExporterImage,
			FluentBitImage:               cfg.FluentBitImage,
			ChownInitContainerImage:      cfg.AlpineImage,
			OTelCollectorImage:           cfg.OTelCollectorImage,
			FluentBitPriorityClassName:   highPriorityClassName,
			LogGatewayPriorityClassName:  normalPriorityClassName,
			LogAgentPriorityClassName:    highPriorityClassName,
			OTLPGatewayPriorityClassName: normalPriorityClassName,
			RestConfig:                   mgr.GetConfig(),
		},
		mgr.GetClient(),
		reconcileTriggerChan,
	)
	if err != nil {
		return fmt.Errorf("failed to create logpipeline controller: %w", err)
	}

	if err := logPipelineController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup logpipeline controller: %w", err)
	}

	return nil
}

func setupTracePipelineController(globals config.Global, envCfg envConfig, mgr manager.Manager, reconcileTriggerChan <-chan event.GenericEvent) error {
	setupLog.Info("Setting up tracepipeline controller")

	tracePipelineController, err := telemetrycontrollers.NewTracePipelineController(
		telemetrycontrollers.TracePipelineControllerConfig{
			Global:                        globals,
			RestConfig:                    mgr.GetConfig(),
			OTelCollectorImage:            envCfg.OTelCollectorImage,
			TraceGatewayPriorityClassName: normalPriorityClassName,
		},
		mgr.GetClient(),
		reconcileTriggerChan,
	)
	if err != nil {
		return fmt.Errorf("failed to create tracepipeline controller: %w", err)
	}

	if err := tracePipelineController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup tracepipeline controller: %w", err)
	}

	return nil
}

func setupMetricPipelineController(globals config.Global, cfg envConfig, mgr manager.Manager, reconcileTriggerChan <-chan event.GenericEvent) error {
	setupLog.Info("Setting up metricpipeline controller")

	metricPipelineController, err := telemetrycontrollers.NewMetricPipelineController(
		telemetrycontrollers.MetricPipelineControllerConfig{
			Global:                         globals,
			MetricAgentPriorityClassName:   highPriorityClassName,
			MetricGatewayPriorityClassName: normalPriorityClassName,
			OTelCollectorImage:             cfg.OTelCollectorImage,
			RestConfig:                     mgr.GetConfig(),
		},
		mgr.GetClient(),
		reconcileTriggerChan,
	)
	if err != nil {
		return fmt.Errorf("failed to create metricpipeline controller: %w", err)
	}

	if err := metricPipelineController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup metricpipeline controller: %w", err)
	}

	return nil
}

func setupConversionWebhooks(mgr manager.Manager) error {
	setupLog.Info("Registering conversion webhooks for LogPipelines")

	if err := ctrl.NewWebhookManagedBy(mgr, &telemetryv1alpha1.LogPipeline{}).Complete(); err != nil {
		return fmt.Errorf("failed to create v1alpha1 conversion webhook: %w", err)
	}

	if err := ctrl.NewWebhookManagedBy(mgr, &telemetryv1beta1.LogPipeline{}).Complete(); err != nil {
		return fmt.Errorf("failed to create v1beta1 conversion webhook: %w", err)
	}

	setupLog.Info("Registering conversion webhooks for MetricPipelines")

	if err := ctrl.NewWebhookManagedBy(mgr, &telemetryv1alpha1.MetricPipeline{}).Complete(); err != nil {
		return fmt.Errorf("failed to create v1alpha1 conversion webhook: %w", err)
	}

	if err := ctrl.NewWebhookManagedBy(mgr, &telemetryv1beta1.MetricPipeline{}).Complete(); err != nil {
		return fmt.Errorf("failed to create v1beta1 conversion webhook: %w", err)
	}

	return nil
}

func ensureWebhookCert(webhookCertConfig webhookcert.Config, mgr manager.Manager) error {
	// Create own client since manager might not be started while using
	clientOptions := client.Options{
		Scheme: scheme,
	}

	k8sClient, err := client.New(mgr.GetConfig(), clientOptions)
	if err != nil {
		return fmt.Errorf("failed to create webhook client: %w", err)
	}

	if err = webhookcert.EnsureCertificate(context.Background(), k8sClient, webhookCertConfig); err != nil {
		return fmt.Errorf("failed to ensure webhook cert: %w", err)
	}

	setupLog.Info("Ensured webhook cert")

	return nil
}

func setNamespaceFieldSelector(globals config.Global) fields.Selector {
	return fields.SelectorFromSet(fields.Set{"metadata.namespace": globals.TargetNamespace()})
}

func setConfigMapNamespaceFieldSelector(globals config.Global) map[string]cache.Config {
	return map[string]cache.Config{
		"kube-system": {
			FieldSelector: fields.SelectorFromSet(fields.Set{"metadata.name": "shoot-info"}),
		},
		globals.TargetNamespace(): {},
	}
}

func createWebhookConfig(globals config.Global) webhookcert.Config {
	return webhookcert.NewWebhookCertConfig(
		webhookcert.ConfigOptions{
			CertDir: certDir,
			ServiceName: types.NamespacedName{
				Name:      webhookServiceName,
				Namespace: globals.ManagerNamespace(),
			},
			CASecretName: types.NamespacedName{
				Name:      names.ManagerWebhookCertSecret,
				Namespace: globals.TargetNamespace(),
			},
			ValidatingWebhookName: types.NamespacedName{
				Name: names.ValidatingWebhookConfig,
			},
			MutatingWebhookName: types.NamespacedName{
				Name: names.MutatingWebhookConfig,
			},
		},
	)
}

// secretCacheTransform removes the Data, StringData, Annotations, and Labels fields from the Secret object before caching it.
// This is done to reduce memory usage and anyway the client cache is disabled for Secrets which means read requests will always go to the API server.
func secretCacheTransform(object any) (any, error) {
	secret, ok := object.(*corev1.Secret)
	if !ok {
		return nil, fmt.Errorf("expected Secret object but got: %T", object)
	}

	secret.Data = nil
	secret.StringData = nil
	secret.Annotations = nil
	secret.Labels = nil

	return secret, nil
}
