package telemetry

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/k8sclients"
	"github.com/kyma-project/telemetry-manager/internal/metrics"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/selfmonitor"
	selfmonitorconfig "github.com/kyma-project/telemetry-manager/internal/selfmonitor/config"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
	telemetryutils "github.com/kyma-project/telemetry-manager/internal/utils/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/webhookcert"
)

const (
	finalizer                    = "telemetry.kyma-project.io/finalizer"
	selfMonitorConfigPath        = "/etc/prometheus/"
	selfMonitorConfigFileName    = "prometheus.yml"
	selfMonitorAlertRuleFileName = "alerting_rules.yml"
)

type Config struct {
	config.Global

	WebhookCert                       webhookcert.Config
	SelfMonitorAlertmanagerWebhookURL string
}

type healthCheckers struct {
	logs, metrics, traces ComponentHealthChecker
}

type OverridesHandler interface {
	LoadOverrides(ctx context.Context) (*overrides.Config, error)
}

type VpaStatusChecker interface {
	VpaCRDExists(ctx context.Context, client client.Client) (bool, error)
}

type NodeSizeTracker interface {
	VPAMaxAllowedMemory() resource.Quantity
	SelfMonitorVPAMaxAllowedMemory() resource.Quantity
}

type SelfMonitorApplierDeleter interface {
	ApplyResources(ctx context.Context, c client.Client, opts selfmonitor.ApplyOptions) error
	DeleteResources(ctx context.Context, c client.Client, vpaCRDExists bool) error
}

type Reconciler struct {
	client.Client

	config Config
	scheme *runtime.Scheme

	healthCheckers            healthCheckers
	overridesHandler          OverridesHandler
	selfMonitorApplierDeleter SelfMonitorApplierDeleter
	vpaStatusChecker          VpaStatusChecker
	nodeSizeTracker           NodeSizeTracker
}

type Option func(*Reconciler)

func New(
	config Config,
	scheme *runtime.Scheme,
	client client.Client,
	overridesHandler OverridesHandler,
	selfMonitorApplierDeleter SelfMonitorApplierDeleter,
	opts ...Option,
) *Reconciler {
	r := &Reconciler{
		config: config,
		scheme: scheme,
		Client: client,
		healthCheckers: healthCheckers{
			logs:    &logComponentsChecker{client: client},
			traces:  &traceComponentsChecker{client: client},
			metrics: &metricComponentsChecker{client: client},
		},
		overridesHandler:          overridesHandler,
		selfMonitorApplierDeleter: selfMonitorApplierDeleter,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logf.FromContext(ctx).V(1).Info("Reconciling Telemetry")

	overrideConfig, err := r.overridesHandler.LoadOverrides(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideConfig.Telemetry.Paused {
		logf.FromContext(ctx).V(1).Info("Skipping reconciliation: paused using override config")
		return ctrl.Result{}, nil
	}

	var telemetry operatorv1beta1.Telemetry
	if err := r.Get(ctx, req.NamespacedName, &telemetry); err != nil {
		logf.FromContext(ctx).Info(req.String() + " got deleted!")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	err = r.doReconcile(ctx, &telemetry, overrideConfig.Global.LogLevel)
	if statusErr := r.updateStatus(ctx, &telemetry); statusErr != nil {
		if err != nil {
			err = fmt.Errorf("failed while updating status: %w: %w", statusErr, err)
		} else {
			err = fmt.Errorf("failed to update status: %w", statusErr)
		}
	}

	requeue := telemetry.Status.State == operatorv1beta1.StateWarning

	return ctrl.Result{Requeue: requeue}, err
}

func (r *Reconciler) doReconcile(ctx context.Context, telemetry *operatorv1beta1.Telemetry, logLevel string) error {
	r.trackServiceAttributesEnrichmentStrategy(ctx)

	if err := r.handleFinalizer(ctx, telemetry); err != nil {
		return fmt.Errorf("failed to manage finalizer: %w", err)
	}

	if err := r.reconcileWebhook(ctx, telemetry); err != nil {
		return fmt.Errorf("failed to reconcile webhook: %w", err)
	}

	if err := r.reconcileSelfMonitor(ctx, telemetry, logLevel); err != nil {
		return fmt.Errorf("failed to reconcile self-monitor deployment: %w", err)
	}

	return nil
}

func (r *Reconciler) reconcileSelfMonitor(ctx context.Context, telemetry *operatorv1beta1.Telemetry, logLevel string) error {
	pipelinesPresent, err := r.checkPipelineExist(ctx)
	if err != nil {
		return err
	}

	vpaCRDExists, err := r.vpaStatusChecker.VpaCRDExists(ctx, r.Client)
	if err != nil {
		return fmt.Errorf("failed to check VPA CRD existence: %w", err)
	}

	if !pipelinesPresent {
		if err := r.selfMonitorApplierDeleter.DeleteResources(ctx, r.Client, vpaCRDExists); err != nil {
			return fmt.Errorf("failed to delete self-monitor resources: %w", err)
		}

		return nil
	}

	prometheusConfig := selfmonitorconfig.MakeConfig(selfmonitorconfig.BuilderConfig{
		ScrapeNamespace:        r.config.TargetNamespace(),
		AlertmanagerWebhookURL: r.config.SelfMonitorAlertmanagerWebhookURL,
		ConfigPath:             selfMonitorConfigPath,
		AlertRuleFileName:      selfMonitorAlertRuleFileName,
	})

	prometheusConfigYAML, err := yaml.Marshal(prometheusConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal selfmonitor config: %w", err)
	}

	alertRules := selfmonitorconfig.MakeRules()

	alertRulesYAML, err := yaml.Marshal(alertRules)
	if err != nil {
		return fmt.Errorf("failed to marshal rules: %w", err)
	}

	isVpaEnabled := telemetryutils.IsVpaEnabledInTelemetry(ctx, r.Client, r.config.DefaultTelemetryNamespace())
	vpaMaxAllowedMemory := r.nodeSizeTracker.SelfMonitorVPAMaxAllowedMemory()

	if err := r.selfMonitorApplierDeleter.ApplyResources(
		ctx,
		k8sclients.NewOwnerReferenceSetter(r.Client, telemetry),
		selfmonitor.ApplyOptions{
			AlertRulesFileName:       selfMonitorAlertRuleFileName,
			AlertRulesYAML:           string(alertRulesYAML),
			PrometheusConfigFileName: selfMonitorConfigFileName,
			PrometheusConfigPath:     selfMonitorConfigPath,
			PrometheusConfigYAML:     string(prometheusConfigYAML),
			LogLevel:                 logLevel,
			VpaCRDExists:             vpaCRDExists,
			VpaEnabled:               isVpaEnabled,
			VPAMaxAllowedMemory:      vpaMaxAllowedMemory,
		},
	); err != nil {
		return fmt.Errorf("failed to apply self-monitor resources: %w", err)
	}

	return nil
}

func (r *Reconciler) checkPipelineExist(ctx context.Context) (bool, error) {
	var allLogPipelines telemetryv1beta1.LogPipelineList
	if err := r.List(ctx, &allLogPipelines); err != nil {
		return false, fmt.Errorf("failed to get all log pipelines: %w", err)
	}

	if len(allLogPipelines.Items) > 0 {
		return true, nil
	}

	var allTracePipelines telemetryv1beta1.TracePipelineList
	if err := r.List(ctx, &allTracePipelines); err != nil {
		return false, fmt.Errorf("failed to get all trace pipelines: %w", err)
	}

	if len(allTracePipelines.Items) > 0 {
		return true, nil
	}

	var allMetricPipelines telemetryv1beta1.MetricPipelineList
	if err := r.List(ctx, &allMetricPipelines); err != nil {
		return false, fmt.Errorf("failed to get all metric pipelines: %w", err)
	}

	if len(allMetricPipelines.Items) > 0 {
		return true, nil
	}

	return false, nil
}

func (r *Reconciler) handleFinalizer(ctx context.Context, telemetry *operatorv1beta1.Telemetry) error {
	if telemetry.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(telemetry, finalizer) {
			controllerutil.AddFinalizer(telemetry, finalizer)

			if err := r.Update(ctx, telemetry); err != nil {
				return fmt.Errorf("failed to update telemetry: %w", err)
			}
		}

		return nil
	}

	if controllerutil.ContainsFinalizer(telemetry, finalizer) {
		if r.dependentCRsFound(ctx) {
			// Block deletion of the resource if there are still some dependent resources
			logf.FromContext(ctx).Info("Telemetry CR deletion is blocked because one or more dependent CRs (LogPipeline, MetricPipeline, TracePipeline) still exist")
			return nil
		}

		controllerutil.RemoveFinalizer(telemetry, finalizer)

		if err := r.Update(ctx, telemetry); err != nil {
			return fmt.Errorf("failed to update telemetry: %w", err)
		}
	}

	return nil
}

func (r *Reconciler) reconcileWebhook(ctx context.Context, telemetry *operatorv1beta1.Telemetry) error {
	// We skip webhook reconciliation only if no pipelines are remaining. This avoids the risk of certificate expiration while waiting for deletion.
	if !telemetry.DeletionTimestamp.IsZero() && !r.dependentCRsFound(ctx) {
		return nil
	}

	if err := webhookcert.EnsureCertificate(ctx, r.Client, r.config.WebhookCert); err != nil {
		return fmt.Errorf("failed to reconcile webhook: %w", err)
	}

	var secret corev1.Secret
	if err := r.Get(ctx, r.config.WebhookCert.CASecretName, &secret); err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	if err := controllerutil.SetOwnerReference(telemetry, &secret, r.scheme); err != nil {
		return fmt.Errorf("failed to set owner reference for secret: %w", err)
	}

	if err := k8sutils.CreateOrUpdateSecret(ctx, r.Client, &secret); err != nil {
		return fmt.Errorf("failed to update secret: %w", err)
	}

	var webhook admissionregistrationv1.ValidatingWebhookConfiguration
	if err := r.Get(ctx, r.config.WebhookCert.ValidatingWebhookName, &webhook); err != nil {
		return fmt.Errorf("failed to get webhook: %w", err)
	}

	if err := k8sutils.CreateOrUpdateValidatingWebhookConfiguration(ctx, r.Client, &webhook); err != nil {
		return fmt.Errorf("failed to update webhook: %w", err)
	}

	return nil
}

func (r *Reconciler) trackServiceAttributesEnrichmentStrategy(ctx context.Context) {
	enrichmentStrategy := telemetryutils.GetServiceEnrichmentFromTelemetryOrDefault(ctx, r.Client, r.config.DefaultTelemetryNamespace())

	// Reset all strategies to 0 before setting the current one to 1. This is to ensure that only the active strategy is set to 1
	metrics.ServiceAttributesEnrichmentStrategy.WithLabelValues(commonresources.AnnotationValueTelemetryServiceEnrichmentOtel).Set(0)
	metrics.ServiceAttributesEnrichmentStrategy.WithLabelValues(commonresources.AnnotationValueTelemetryServiceEnrichmentKymaLegacy).Set(0)

	metrics.ServiceAttributesEnrichmentStrategy.WithLabelValues(enrichmentStrategy).Set(1)
}

// WithVpaStatusChecker sets the VPA status checker.
func WithVpaStatusChecker(checker VpaStatusChecker) Option {
	return func(r *Reconciler) {
		r.vpaStatusChecker = checker
	}
}

// WithNodeSizeTracker sets the node size tracker.
func WithNodeSizeTracker(tracker NodeSizeTracker) Option {
	return func(r *Reconciler) {
		r.nodeSizeTracker = tracker
	}
}
