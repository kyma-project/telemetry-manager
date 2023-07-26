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
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
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
	"github.com/kyma-project/telemetry-manager/internal/secretref"
)

type Config struct {
	DaemonSet         types.NamespacedName
	SectionsConfigMap types.NamespacedName
	FilesConfigMap    types.NamespacedName
	LuaConfigMap      types.NamespacedName
	ParsersConfigMap  types.NamespacedName
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

	var allPipelines telemetryv1alpha1.LogPipelineList
	if err := r.List(ctx, &allPipelines); err != nil {
		return fmt.Errorf("failed to get all log pipelines while syncing Fluent Bit ConfigMaps: %v", err)
	}

	if err = ensureFinalizers(ctx, r.Client, pipeline); err != nil {
		return err
	}

	deployableLogPipelines := r.getDeployableLogpipelines(ctx, allPipelines.Items)
	if err = r.syncer.syncFluentBitConfig(ctx, pipeline, allPipelines, deployableLogPipelines); err != nil {
		return err
	}

	if err = r.reconcileFluentBit(ctx, pipeline, deployableLogPipelines); err != nil {
		return err
	}

	if err = cleanupFinalizersIfNeeded(ctx, r.Client, pipeline); err != nil {
		return err
	}

	return err
}

func (r *Reconciler) reconcileFluentBit(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline, deployableLogpipelines []telemetryv1alpha1.LogPipeline) error {
	serviceAccount := commonresources.MakeServiceAccount(r.config.DaemonSet)
	if err := controllerutil.SetOwnerReference(pipeline, serviceAccount, r.Scheme()); err != nil {
		return err
	}
	if err := utils.CreateOrUpdateServiceAccount(ctx, r, serviceAccount); err != nil {
		return fmt.Errorf("failed to create fluent bit service account: %w", err)
	}

	clusterRole := resources.MakeClusterRole(r.config.DaemonSet)
	if err := controllerutil.SetOwnerReference(pipeline, clusterRole, r.Scheme()); err != nil {
		return err
	}
	if err := utils.CreateOrUpdateClusterRole(ctx, r, clusterRole); err != nil {
		return fmt.Errorf("failed to create fluent bit cluster role: %w", err)
	}

	clusterRoleBinding := commonresources.MakeClusterRoleBinding(r.config.DaemonSet)
	if err := controllerutil.SetOwnerReference(pipeline, clusterRoleBinding, r.Scheme()); err != nil {
		return err
	}
	if err := utils.CreateOrUpdateClusterRoleBinding(ctx, r, clusterRoleBinding); err != nil {
		return fmt.Errorf("failed to create fluent bit cluster role Binding: %w", err)
	}

	exporterMetricsService := resources.MakeExporterMetricsService(r.config.DaemonSet)
	if err := controllerutil.SetOwnerReference(pipeline, exporterMetricsService, r.Scheme()); err != nil {
		return err
	}
	if err := utils.CreateOrUpdateService(ctx, r, exporterMetricsService); err != nil {
		return fmt.Errorf("failed to reconcile exporter metrics service: %w", err)
	}

	metricsService := resources.MakeMetricsService(r.config.DaemonSet)
	if err := controllerutil.SetOwnerReference(pipeline, metricsService, r.Scheme()); err != nil {
		return err
	}
	if err := utils.CreateOrUpdateService(ctx, r, metricsService); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit metrics service: %w", err)
	}

	cm := resources.MakeConfigMap(r.config.DaemonSet)
	if len(deployableLogpipelines) == 0 {
		fluentbitConf := cm.Data["fluent-bit.conf"]
		fluentbitConf = strings.ReplaceAll(fluentbitConf, "@INCLUDE dynamic/*.conf", "")
		cm.Data["fluent-bit.conf"] = fluentbitConf
	}
	if err := controllerutil.SetOwnerReference(pipeline, cm, r.Scheme()); err != nil {
		return err
	}
	if err := utils.CreateOrUpdateConfigMap(ctx, r, cm); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit configmap: %w", err)
	}

	luaCm := resources.MakeLuaConfigMap(r.config.LuaConfigMap)
	if err := controllerutil.SetOwnerReference(pipeline, luaCm, r.Scheme()); err != nil {
		return err
	}
	if err := utils.CreateOrUpdateConfigMap(ctx, r, luaCm); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit lua configmap: %w", err)
	}

	parsersCm := resources.MakeParserConfigmap(r.config.ParsersConfigMap)
	if err := controllerutil.SetOwnerReference(pipeline, parsersCm, r.Scheme()); err != nil {
		return err
	}
	if err := utils.CreateIfNotExistsConfigMap(ctx, r, parsersCm); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit parser configmap: %w", err)
	}

	var checksum string
	var err error
	if checksum, err = r.calculateChecksum(ctx); err != nil {
		return fmt.Errorf("failed to calculate config checksum: %w", err)
	}

	daemonSet := resources.MakeDaemonSet(r.config.DaemonSet, checksum, r.config.DaemonSetConfig)
	if err := controllerutil.SetOwnerReference(pipeline, daemonSet, r.Scheme()); err != nil {
		return err
	}
	if err := utils.CreateOrUpdateDaemonSet(ctx, r, daemonSet); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit daemonset: %w", err)
	}

	return nil
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
	var baseCm corev1.ConfigMap
	if err := r.Get(ctx, r.config.DaemonSet, &baseCm); err != nil {
		return "", fmt.Errorf("failed to get %s/%s ConfigMap: %v", r.config.DaemonSet.Namespace, r.config.DaemonSet.Name, err)
	}

	var parsersCm corev1.ConfigMap
	if err := r.Get(ctx, r.config.ParsersConfigMap, &parsersCm); err != nil {
		return "", fmt.Errorf("failed to get %s/%s ConfigMap: %v", r.config.ParsersConfigMap.Namespace, r.config.ParsersConfigMap.Name, err)
	}

	var luaCm corev1.ConfigMap
	if err := r.Get(ctx, r.config.LuaConfigMap, &luaCm); err != nil {
		return "", fmt.Errorf("failed to get %s/%s ConfigMap: %v", r.config.LuaConfigMap.Namespace, r.config.LuaConfigMap.Name, err)
	}

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

	return configchecksum.Calculate([]corev1.ConfigMap{baseCm, parsersCm, luaCm, sectionsCm, filesCm}, []corev1.Secret{envSecret}), nil
}

// isLogPipelineDeployable checks if logpipeline is ready to be rendered into the fluentbit configuration. A pipeline is deployable if it is not being deleted, all secret references exist, and is not above the pipeline limit.
func (r *Reconciler) getDeployableLogpipelines(ctx context.Context, allPipelines []telemetryv1alpha1.LogPipeline) []telemetryv1alpha1.LogPipeline {
	var deployablePipelines []telemetryv1alpha1.LogPipeline
	for i := range allPipelines {
		if !allPipelines[i].GetDeletionTimestamp().IsZero() {
			continue
		}

		if secretref.ReferencesNonExistentSecret(ctx, r.Client, &allPipelines[i]) {
			continue
		}
		deployablePipelines = append(deployablePipelines, allPipelines[i])
	}

	return deployablePipelines
}
