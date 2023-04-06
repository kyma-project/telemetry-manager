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

package logpipeline

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	configbuilder "github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	utils "github.com/kyma-project/telemetry-manager/internal/kubernetes"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	resources "github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"
)

type Config struct {
	DaemonSet         types.NamespacedName
	SectionsConfigMap types.NamespacedName
	FilesConfigMap    types.NamespacedName
	EnvSecret         types.NamespacedName
	OverrideConfigMap types.NamespacedName
	PipelineDefaults  configbuilder.PipelineDefaults
	Overrides         overrides.Config
	DaemonSetConfig   resources.DaemonSetConfig
}

//go:generate mockery --name DaemonSetProber --filename daemon_set_prober.go
type DaemonSetProber interface {
	IsReady(ctx context.Context, name types.NamespacedName) (bool, error)
}

//go:generate mockery --name DaemonSetAnnotator --filename daemon_set_annotator.go
type DaemonSetAnnotator interface {
	SetAnnotation(ctx context.Context, name types.NamespacedName, key, value string) error
}

type Reconciler struct {
	client.Client
	config                  Config
	prober                  DaemonSetProber
	allLogPipelines         prometheus.Gauge
	unsupportedLogPipelines prometheus.Gauge
	syncer                  syncer
	overridesHandler        *overrides.Handler
}

func NewReconciler(client client.Client, config Config, prober DaemonSetProber, overridesHandler *overrides.Handler) *Reconciler {
	var r Reconciler
	r.Client = client
	r.config = config
	r.prober = prober
	r.allLogPipelines = prometheus.NewGauge(prometheus.GaugeOpts{Name: "telemetry_all_logpipelines", Help: "Number of log pipelines."})
	r.unsupportedLogPipelines = prometheus.NewGauge(prometheus.GaugeOpts{Name: "telemetry_unsupported_logpipelines", Help: "Number of log pipelines with custom filters or outputs."})
	metrics.Registry.MustRegister(r.allLogPipelines, r.unsupportedLogPipelines)
	r.syncer = syncer{client, config}
	r.overridesHandler = overridesHandler

	return &r
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.V(1).Info("Reconciliation triggered")

	overrideConfig, err := r.overridesHandler.UpdateOverrideConfig(ctx, r.config.OverrideConfigMap)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.overridesHandler.CheckGlobalConfig(overrideConfig.Global); err != nil {
		return ctrl.Result{}, err
	}

	if overrideConfig.Logging.Paused {
		log.V(1).Info("Skipping reconciliation of logpipeline as reconciliation is paused.")
		return ctrl.Result{}, nil
	}

	if err := r.updateMetrics(ctx); err != nil {
		log.Error(err, "Failed to get all LogPipelines while updating metrics")
	}

	var pipeline telemetryv1alpha1.LogPipeline
	if err := r.Get(ctx, req.NamespacedName, &pipeline); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return ctrl.Result{}, r.doReconcile(ctx, &pipeline)
}

func (r *Reconciler) doReconcile(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) (err error) {
	// defer the updating of status to ensure that the status is updated regardless of the outcome of the reconciliation
	defer func() {
		if statusErr := r.updateStatus(ctx, pipeline.Name); statusErr != nil {
			if err != nil {
				err = fmt.Errorf("failed while updating status: %v: %v", statusErr, err)
			} else {
				err = fmt.Errorf("failed to update status: %v", statusErr)
			}
		}
	}()

	if err = ensureFinalizers(ctx, r.Client, pipeline); err != nil {
		return err
	}

	if err = r.syncer.syncFluentBitConfig(ctx, pipeline); err != nil {
		return err
	}

	var checksum string
	if checksum, err = r.calculateChecksum(ctx); err != nil {
		return err
	}

	name := r.config.DaemonSet
	if err = r.reconcileFluentBit(ctx, name, pipeline, checksum); err != nil {
		return err
	}

	if err = cleanupFinalizersIfNeeded(ctx, r.Client, pipeline); err != nil {
		return err
	}

	return err
}

func (r *Reconciler) reconcileFluentBit(ctx context.Context, name types.NamespacedName, pipeline *telemetryv1alpha1.LogPipeline, checksum string) error {
	serviceAccount := commonresources.MakeServiceAccount(name)
	if err := setOwnerReference(pipeline, serviceAccount, r.Scheme()); err != nil {
		return err
	}
	if err := utils.CreateOrUpdateServiceAccount(ctx, r, serviceAccount); err != nil {
		return fmt.Errorf("failed to create fluent bit service account: %w", err)
	}

	clusterRole := commonresources.MakeClusterRole(name)
	if err := setOwnerReference(pipeline, clusterRole, r.Scheme()); err != nil {
		return err
	}
	if err := utils.CreateOrUpdateClusterRole(ctx, r, clusterRole); err != nil {
		return fmt.Errorf("failed to create fluent bit cluster role: %w", err)
	}

	clusterRoleBinding := commonresources.MakeClusterRoleBinding(name)
	if err := setOwnerReference(pipeline, clusterRoleBinding, r.Scheme()); err != nil {
		return err
	}
	if err := utils.CreateOrUpdateClusterRoleBinding(ctx, r, clusterRoleBinding); err != nil {
		return fmt.Errorf("failed to create fluent bit cluster role Binding: %w", err)
	}

	daemonSet := resources.MakeDaemonSet(name, checksum, r.config.DaemonSetConfig)
	if err := setOwnerReference(pipeline, daemonSet, r.Scheme()); err != nil {
		return err
	}
	if err := utils.CreateOrUpdateDaemonSet(ctx, r, daemonSet); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit daemonset: %w", err)
	}

	exporterMetricsService := resources.MakeExporterMetricsService(name)
	if err := setOwnerReference(pipeline, exporterMetricsService, r.Scheme()); err != nil {
		return err
	}
	if err := utils.CreateOrUpdateService(ctx, r, exporterMetricsService); err != nil {
		return fmt.Errorf("failed to reconcile exporter metrics service: %w", err)
	}

	metricsService := resources.MakeMetricsService(name)
	if err := setOwnerReference(pipeline, metricsService, r.Scheme()); err != nil {
		return err
	}
	if err := utils.CreateOrUpdateService(ctx, r, metricsService); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit metrics service: %w", err)
	}

	cm := resources.MakeConfigMap(name)
	if err := setOwnerReference(pipeline, cm, r.Scheme()); err != nil {
		return err
	}
	if err := utils.CreateOrUpdateConfigMap(ctx, r, cm); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit configmap: %w", err)
	}

	luaCm := resources.MakeLuaConfigMap(name)
	if err := setOwnerReference(pipeline, luaCm, r.Scheme()); err != nil {
		return err
	}
	if err := utils.CreateOrUpdateConfigMap(ctx, r, luaCm); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit lua configmap: %w", err)
	}

	parsersCm := resources.MakeDynamicParserConfigmap(name)
	if err := setOwnerReference(pipeline, parsersCm, r.Scheme()); err != nil {
		return err
	}
	if err := utils.CreateIfNotExistsConfigMap(ctx, r, parsersCm); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit parser configmap: %w", err)
	}
	return nil
}

func setOwnerReference(owner, object metav1.Object, scheme *runtime.Scheme) error {
	err := controllerutil.SetControllerReference(owner, object, scheme)
	if err != nil {
		err = controllerutil.SetOwnerReference(owner, object, scheme)
	}
	return err
}

func deleteFluentBit(ctx context.Context, c client.Client, name types.NamespacedName) error {
	if err := deleteResource(ctx, c, name, &appsv1.DaemonSet{}); err != nil {
		return fmt.Errorf("unable to delete daemonset %s: %v", name.Name, err)
	}

	if err := deleteResource(ctx, c, name, &corev1.Service{}); err != nil {
		return fmt.Errorf("unable to delete service %s: %v", name.Name, err)
	}

	if err := deleteResource(ctx, c, name, &corev1.ConfigMap{}); err != nil {
		return fmt.Errorf("unable to delete configmap %s: %v", name.Name, err)
	}

	if err := deleteResource(ctx, c, name, &corev1.ServiceAccount{}); err != nil {
		return fmt.Errorf("unable to delete service account %s: %v", name.Name, err)
	}

	if err := deleteResource(ctx, c, name, &rbacv1.ClusterRoleBinding{}); err != nil {
		return fmt.Errorf("unable to delete cluster role binding %s: %v", name.Name, err)
	}

	if err := deleteResource(ctx, c, name, &rbacv1.ClusterRole{}); err != nil {
		return fmt.Errorf("unable to delete cluster role %s: %v", name.Name, err)
	}

	name.Name = fmt.Sprintf("%s-luascripts", name.Name)
	if err := deleteResource(ctx, c, name, &corev1.ConfigMap{}); err != nil {
		return fmt.Errorf("unable to delete configmap %s: %v", name.Name, err)
	}

	return nil
}

func deleteResource(ctx context.Context, c client.Client, name client.ObjectKey, obj client.Object) error {
	err := c.Get(ctx, name, obj)
	if err == nil {
		if err = c.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	} else if !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (r *Reconciler) isLastPipelineMarkedForDeletion(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) (bool, error) {
	if isNotMarkedForDeletion(pipeline) {
		return false, nil
	}

	var allPipelines telemetryv1alpha1.LogPipelineList
	if err := r.List(ctx, &allPipelines); err != nil {
		return false, fmt.Errorf("failed to list LogPipelines: %v", err)
	}

	return len(allPipelines.Items) == 1 && allPipelines.Items[0].Name == pipeline.Name, nil
}

func (r *Reconciler) updateMetrics(ctx context.Context) error {
	var allPipelines telemetryv1alpha1.LogPipelineList
	if err := r.List(ctx, &allPipelines); err != nil {
		return err
	}

	r.allLogPipelines.Set(float64(count(&allPipelines, isNotMarkedForDeletion)))
	r.unsupportedLogPipelines.Set(float64(count(&allPipelines, isUnsupported)))

	return nil
}

type keepFunc func(*telemetryv1alpha1.LogPipeline) bool

func count(pipelines *telemetryv1alpha1.LogPipelineList, keep keepFunc) int {
	c := 0
	for i := range pipelines.Items {
		if keep(&pipelines.Items[i]) {
			c++
		}
	}
	return c
}

func isNotMarkedForDeletion(pipeline *telemetryv1alpha1.LogPipeline) bool {
	return pipeline.ObjectMeta.DeletionTimestamp.IsZero()
}

func isUnsupported(pipeline *telemetryv1alpha1.LogPipeline) bool {
	return isNotMarkedForDeletion(pipeline) && pipeline.ContainsCustomPlugin()
}

func (r *Reconciler) calculateChecksum(ctx context.Context) (string, error) {
	var sectionsCm corev1.ConfigMap
	if err := r.Get(ctx, r.config.SectionsConfigMap, &sectionsCm); err != nil {
		return "", fmt.Errorf("failed to get %s/%s ConfigMap: %v", r.config.SectionsConfigMap.Namespace, r.config.SectionsConfigMap.Name, err)
	}

	var filesCm corev1.ConfigMap
	if err := r.Get(ctx, r.config.FilesConfigMap, &filesCm); err != nil {
		return "", fmt.Errorf("failed to get %s/%s ConfigMap: %v", r.config.FilesConfigMap.Namespace, r.config.FilesConfigMap.Name, err)
	}

	var envSecret corev1.Secret
	if err := r.Get(ctx, r.config.EnvSecret, &envSecret); err != nil {
		return "", fmt.Errorf("failed to get %s/%s ConfigMap: %v", r.config.EnvSecret.Namespace, r.config.EnvSecret.Name, err)
	}

	return configchecksum.Calculate([]corev1.ConfigMap{sectionsCm, filesCm}, []corev1.Secret{envSecret}), nil
}
