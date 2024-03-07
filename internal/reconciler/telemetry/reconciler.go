package telemetry

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/k8sutils"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/resources/selfmonitor"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/config"
	"github.com/kyma-project/telemetry-manager/internal/webhookcert"
)

const (
	finalizer = "telemetry.kyma-project.io/finalizer"
)

type Config struct {
	Traces                 TracesConfig
	Metrics                MetricsConfig
	Webhook                WebhookConfig
	OverridesConfigMapName types.NamespacedName
	SelfMonitor            SelfMonitorConfig
}

type TracesConfig struct {
	OTLPServiceName string
	Namespace       string
}

type MetricsConfig struct {
	OTLPServiceName string
	Namespace       string
}

type WebhookConfig struct {
	Enabled    bool
	CertConfig webhookcert.Config
}

type SelfMonitorConfig struct {
	Enabled bool
	Config  selfmonitor.Config
}

type healthCheckers struct {
	logs, metrics, traces ComponentHealthChecker
}

type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
	*rest.Config
	config           Config
	healthCheckers   healthCheckers
	overridesHandler *overrides.Handler
}

func NewReconciler(client client.Client, scheme *runtime.Scheme, config Config, overridesHandler *overrides.Handler) *Reconciler {
	return &Reconciler{
		Client: client,
		Scheme: scheme,
		config: config,
		healthCheckers: healthCheckers{
			logs:    &logComponentsChecker{client: client},
			traces:  &traceComponentsChecker{client: client},
			metrics: &metricComponentsChecker{client: client},
		},
		overridesHandler: overridesHandler,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logf.FromContext(ctx).V(1).Info("Reconciling")

	overrideConfig, err := r.overridesHandler.LoadOverrides(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideConfig.Telemetry.Paused {
		logf.FromContext(ctx).V(1).Info("Skipping reconciliation: paused using override config")
		return ctrl.Result{}, nil
	}

	if err := r.cleanUpOldNetworkPolicies(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to clean up old network policies: %w", err)
	}

	var telemetry operatorv1alpha1.Telemetry
	if err := r.Client.Get(ctx, req.NamespacedName, &telemetry); err != nil {
		logf.FromContext(ctx).Info(req.NamespacedName.String() + " got deleted!")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.handleFinalizer(ctx, &telemetry); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to manage finalizer: %w", err)
	}

	if err := r.updateStatus(ctx, &telemetry); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update status: %w", err)
	}

	if err := r.reconcileWebhook(ctx, &telemetry); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile webhook: %w", err)
	}

	if err = r.reconcileSelfMonitor(ctx, telemetry); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile self-monitor deployment: %w", err)
	}

	requeue := telemetry.Status.State == operatorv1alpha1.StateWarning
	return ctrl.Result{Requeue: requeue}, nil
}

func (r *Reconciler) reconcileSelfMonitor(ctx context.Context, telemetry operatorv1alpha1.Telemetry) error {
	if !r.config.SelfMonitor.Enabled {
		return nil
	}

	pipelinesPresent, err := r.checkPipelineExist(ctx)
	if err != nil {
		return err
	}
	if !pipelinesPresent {
		if err := selfmonitor.RemoveResources(ctx, r.Client, &r.config.SelfMonitor.Config); err != nil {
			return fmt.Errorf("failed to delete self-monitor resources: %w", err)
		}
		return nil
	}

	selfMonConfig := config.MakeConfig()
	selfMonitorConfigYaml, err := yaml.Marshal(selfMonConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal selfmonitor config: %w", err)
	}
	r.config.SelfMonitor.Config.SelfMonitorConfig = string(selfMonitorConfigYaml)

	if err := selfmonitor.ApplyResources(ctx,
		k8sutils.NewOwnerReferenceSetter(r.Client, &telemetry),
		&r.config.SelfMonitor.Config); err != nil {
		return fmt.Errorf("failed to apply self-monitor resources: %w", err)
	}

	return nil
}

func (r *Reconciler) checkPipelineExist(ctx context.Context) (bool, error) {
	var allLogPipelines telemetryv1alpha1.LogPipelineList
	if err := r.List(ctx, &allLogPipelines); err != nil {
		return false, fmt.Errorf("failed to get all log pipelines: %w", err)
	}
	if len(allLogPipelines.Items) > 0 {
		return true, nil
	}

	var allTracePipelines telemetryv1alpha1.TracePipelineList
	if err := r.List(ctx, &allTracePipelines); err != nil {
		return false, fmt.Errorf("failed to get all trace pipelines: %w", err)
	}
	if len(allTracePipelines.Items) > 0 {
		return true, nil
	}

	var allMetricPipelines telemetryv1alpha1.MetricPipelineList
	if err := r.List(ctx, &allMetricPipelines); err != nil {
		return false, fmt.Errorf("failed to get all metric pipelines: %w", err)
	}
	if len(allMetricPipelines.Items) > 0 {
		return true, nil
	}

	return false, nil
}

func (r *Reconciler) handleFinalizer(ctx context.Context, telemetry *operatorv1alpha1.Telemetry) error {
	if telemetry.ObjectMeta.DeletionTimestamp.IsZero() {
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
			logf.FromContext(ctx).Info("Telemetry CR deletion is blocked because one or more dependent CRs (LogPipeline, LogParser, MetricPipeline, TracePipeline) still exist")
			return nil
		}

		err := r.deleteWebhook(ctx)
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete webhook: %w", err)
		}

		controllerutil.RemoveFinalizer(telemetry, finalizer)
		if err := r.Update(ctx, telemetry); err != nil {
			return fmt.Errorf("failed to update telemetry: %w", err)
		}
	}

	return nil
}

func (r *Reconciler) deleteWebhook(ctx context.Context) error {
	webhook := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: r.config.Webhook.CertConfig.WebhookName.Name,
		},
	}

	return r.Delete(ctx, webhook)
}

func (r *Reconciler) reconcileWebhook(ctx context.Context, telemetry *operatorv1alpha1.Telemetry) error {
	if !r.config.Webhook.Enabled {
		return nil
	}

	// We skip webhook reconciliation only if no pipelines are remaining. This avoids the risk of certificate expiration while waiting for deletion.
	if !telemetry.DeletionTimestamp.IsZero() && !r.dependentCRsFound(ctx) {
		return nil
	}

	if err := webhookcert.EnsureCertificate(ctx, r.Client, r.config.Webhook.CertConfig); err != nil {
		return fmt.Errorf("failed to reconcile webhook: %w", err)
	}

	var secret corev1.Secret
	if err := r.Get(ctx, r.config.Webhook.CertConfig.CASecretName, &secret); err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}
	if err := controllerutil.SetOwnerReference(telemetry, &secret, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference for secret: %w", err)
	}
	if err := k8sutils.CreateOrUpdateSecret(ctx, r.Client, &secret); err != nil {
		return fmt.Errorf("failed to update secret: %w", err)
	}

	var webhook admissionregistrationv1.ValidatingWebhookConfiguration
	if err := r.Get(ctx, r.config.Webhook.CertConfig.WebhookName, &webhook); err != nil {
		return fmt.Errorf("failed to get webhook: %w", err)
	}
	if err := k8sutils.CreateOrUpdateValidatingWebhookConfiguration(ctx, r.Client, &webhook); err != nil {
		return fmt.Errorf("failed to update webhook: %w", err)
	}

	return nil
}

func (r *Reconciler) cleanUpOldNetworkPolicies(ctx context.Context) error {
	oldNetworkPoliciesNames := []string{
		"telemetry-manager-pprof-deny-ingress",
		"telemetry-metric-gateway-pprof-deny-ingress",
		"telemetry-metric-agent-pprof-deny-ingress",
		"telemetry-trace-collector-pprof-deny-ingress",
	}
	for _, networkPolicyName := range oldNetworkPoliciesNames {
		networkPolicy := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      networkPolicyName,
				Namespace: r.config.OverridesConfigMapName.Namespace,
			},
		}
		if err := r.Delete(ctx, networkPolicy); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("failed to delete old network policy %s in namespace %s: %w", networkPolicyName, r.config.OverridesConfigMapName.Namespace, err)
		}
	}
	return nil
}
