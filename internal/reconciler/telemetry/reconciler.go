package telemetry

import (
	"context"
	"fmt"
	"time"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/kubernetes"
	"github.com/kyma-project/telemetry-manager/internal/webhookcert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	requeueInterval = time.Second * 10
	finalizer       = "telemetry.kyma-project.io/finalizer"
	fieldOwner      = "telemetry.kyma-project.io/owner"
)

type Config struct {
	Traces  TracesConfig
	Metrics MetricsConfig
	Webhook WebhookConfig
}

type TracesConfig struct {
	OTLPServiceName string
	Namespace       string
}

type MetricsConfig struct {
	Enabled         bool
	OTLPServiceName string
	Namespace       string
}

type WebhookConfig struct {
	Enabled    bool
	CertConfig webhookcert.Config
}

type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
	*rest.Config
	// EventRecorder for creating k8s events
	record.EventRecorder
	config         Config
	healthCheckers []ComponentHealthChecker
}

func NewReconciler(client client.Client, scheme *runtime.Scheme, eventRecorder record.EventRecorder, config Config) *Reconciler {
	healthCheckers := []ComponentHealthChecker{
		&logComponentsChecker{client: client},
		&traceComponentsChecker{client: client},
	}
	if config.Metrics.Enabled {
		healthCheckers = append(healthCheckers, &metricComponentsChecker{client: client})
	}

	return &Reconciler{
		Client:         client,
		Scheme:         scheme,
		EventRecorder:  eventRecorder,
		config:         config,
		healthCheckers: healthCheckers,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var telemetry operatorv1alpha1.Telemetry
	if err := r.Client.Get(ctx, req.NamespacedName, &telemetry); err != nil {
		logger.Info(req.NamespacedName.String() + " got deleted!")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.handleFinalizer(ctx, &telemetry); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to manage finalizer")
	}

	if err := r.updateStatus(ctx, &telemetry); err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("failed to update status")
	}

	if err := r.reconcileWebhook(ctx, &telemetry); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile webhook: %w", err)
	}

	return ctrl.Result{}, nil
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
		if r.dependentResourcesFound(ctx) {
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
	webhook := &admissionv1.ValidatingWebhookConfiguration{
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

	if !telemetry.DeletionTimestamp.IsZero() {
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
	if err := kubernetes.CreateOrUpdateSecret(ctx, r.Client, &secret); err != nil {
		return fmt.Errorf("failed to update secret: %w", err)
	}

	var webhook admissionv1.ValidatingWebhookConfiguration
	if err := r.Get(ctx, r.config.Webhook.CertConfig.WebhookName, &webhook); err != nil {
		return fmt.Errorf("failed to get webhook: %w", err)
	}
	if err := kubernetes.CreateOrUpdateValidatingWebhookConfiguration(ctx, r.Client, &webhook); err != nil {
		return fmt.Errorf("failed to update webhook: %w", err)
	}

	return nil
}

func (r *Reconciler) serverSideApply(ctx context.Context, obj client.Object) error {
	obj.SetManagedFields(nil)
	obj.SetResourceVersion("")
	return r.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner(fieldOwner))
}
