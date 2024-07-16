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
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/ports"
	"github.com/kyma-project/telemetry-manager/internal/k8sutils"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	"github.com/kyma-project/telemetry-manager/internal/tlscert"
)

type Config struct {
	DaemonSet             types.NamespacedName
	SectionsConfigMap     types.NamespacedName
	FilesConfigMap        types.NamespacedName
	LuaConfigMap          types.NamespacedName
	ParsersConfigMap      types.NamespacedName
	EnvSecret             types.NamespacedName
	OutputTLSConfigSecret types.NamespacedName
	OverrideConfigMap     types.NamespacedName
	PipelineDefaults      builder.PipelineDefaults
	Overrides             overrides.Config
	DaemonSetConfig       fluentbit.DaemonSetConfig
}

type DaemonSetProber interface {
	IsReady(ctx context.Context, name types.NamespacedName) (bool, error)
}

type DaemonSetAnnotator interface {
	SetAnnotation(ctx context.Context, name types.NamespacedName, key, value string) error
}

type FlowHealthProber interface {
	Probe(ctx context.Context, pipelineName string) (prober.LogPipelineProbeResult, error)
}

type OverridesHandler interface {
	LoadOverrides(ctx context.Context) (*overrides.Config, error)
}

type IstioStatusChecker interface {
	IsIstioActive(ctx context.Context) bool
}

type Reconciler struct {
	client.Client

	config Config
	syncer syncer

	// Dependencies
	agentProber        DaemonSetProber
	flowHealthProber   FlowHealthProber
	istioStatusChecker IstioStatusChecker
	overridesHandler   OverridesHandler
	pipelineValidator  pipelineValidator
}

func New(
	client client.Client,
	config Config,
	agentProber DaemonSetProber,
	flowHealthProber FlowHealthProber,
	istioStatusChecker IstioStatusChecker,
	overridesHandler OverridesHandler,
	tlsCertValidator TLSCertValidator,
) *Reconciler {
	return &Reconciler{
		Client: client,
		config: config,
		syncer: syncer{client, config},

		agentProber:        agentProber,
		flowHealthProber:   flowHealthProber,
		istioStatusChecker: istioStatusChecker,
		overridesHandler:   overridesHandler,
		pipelineValidator: pipelineValidator{
			client:           client,
			tlsCertValidator: tlsCertValidator,
		},
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logf.FromContext(ctx).V(1).Info("Reconciling")

	overrideConfig, err := r.overridesHandler.LoadOverrides(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideConfig.Logging.Paused {
		logf.FromContext(ctx).V(1).Info("Skipping reconciliation: paused using override config")
		return ctrl.Result{}, nil
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
				err = fmt.Errorf("failed while updating status: %w: %w", statusErr, err)
			} else {
				err = fmt.Errorf("failed to update status: %w", statusErr)
			}
		}
	}()

	var allPipelines telemetryv1alpha1.LogPipelineList
	if err := r.List(ctx, &allPipelines); err != nil {
		return fmt.Errorf("failed to get all log pipelines while syncing Fluent Bit ConfigMaps: %w", err)
	}

	if err = ensureFinalizers(ctx, r.Client, pipeline); err != nil {
		return err
	}

	reconcilablePipelines, err := r.getReconcilablePipelines(ctx, allPipelines.Items)
	if err != nil {
		return fmt.Errorf("failed to fetch deployable log pipelines: %w", err)
	}
	if len(reconcilablePipelines) == 0 {
		logf.FromContext(ctx).V(1).Info("cleaning up log pipeline resources: all log pipelines are non-reconcilable")
		if err = r.deleteResources(ctx); err != nil {
			return fmt.Errorf("failed to delete log pipeline resources: %w", err)
		}
	}

	if err = r.syncer.syncFluentBitConfig(ctx, pipeline, reconcilablePipelines); err != nil {
		return err
	}

	if err = r.reconcileFluentBit(ctx, pipeline, reconcilablePipelines); err != nil {
		return err
	}

	if err = cleanupFinalizersIfNeeded(ctx, r.Client, pipeline); err != nil {
		return err
	}

	return err
}

func (r *Reconciler) reconcileFluentBit(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline, pipelines []telemetryv1alpha1.LogPipeline) error {
	if len(pipelines) == 0 {
		return nil
	}
	ownerRefSetter := k8sutils.NewOwnerReferenceSetter(r.Client, pipeline)

	serviceAccount := commonresources.MakeServiceAccount(r.config.DaemonSet)
	if err := k8sutils.CreateOrUpdateServiceAccount(ctx, ownerRefSetter, serviceAccount); err != nil {
		return fmt.Errorf("failed to create fluent bit service account: %w", err)
	}

	clusterRole := fluentbit.MakeClusterRole(r.config.DaemonSet)
	if err := k8sutils.CreateOrUpdateClusterRole(ctx, ownerRefSetter, clusterRole); err != nil {
		return fmt.Errorf("failed to create fluent bit cluster role: %w", err)
	}

	clusterRoleBinding := commonresources.MakeClusterRoleBinding(r.config.DaemonSet)
	if err := k8sutils.CreateOrUpdateClusterRoleBinding(ctx, ownerRefSetter, clusterRoleBinding); err != nil {
		return fmt.Errorf("failed to create fluent bit cluster role Binding: %w", err)
	}

	exporterMetricsService := fluentbit.MakeExporterMetricsService(r.config.DaemonSet)
	if err := k8sutils.CreateOrUpdateService(ctx, ownerRefSetter, exporterMetricsService); err != nil {
		return fmt.Errorf("failed to reconcile exporter metrics service: %w", err)
	}

	metricsService := fluentbit.MakeMetricsService(r.config.DaemonSet)
	if err := k8sutils.CreateOrUpdateService(ctx, ownerRefSetter, metricsService); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit metrics service: %w", err)
	}

	cm := fluentbit.MakeConfigMap(r.config.DaemonSet)
	if err := k8sutils.CreateOrUpdateConfigMap(ctx, ownerRefSetter, cm); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit configmap: %w", err)
	}

	luaCm := fluentbit.MakeLuaConfigMap(r.config.LuaConfigMap)
	if err := k8sutils.CreateOrUpdateConfigMap(ctx, ownerRefSetter, luaCm); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit lua configmap: %w", err)
	}

	parsersCm := fluentbit.MakeParserConfigmap(r.config.ParsersConfigMap)
	if err := k8sutils.CreateIfNotExistsConfigMap(ctx, ownerRefSetter, parsersCm); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit parser configmap: %w", err)
	}

	var checksum string
	var err error
	if checksum, err = r.calculateChecksum(ctx); err != nil {
		return fmt.Errorf("failed to calculate config checksum: %w", err)
	}

	daemonSet := fluentbit.MakeDaemonSet(r.config.DaemonSet, checksum, r.config.DaemonSetConfig)
	if err := k8sutils.CreateOrUpdateDaemonSet(ctx, ownerRefSetter, daemonSet); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit daemonset: %w", err)
	}

	allowedPorts := getFluentBitPorts()
	if r.istioStatusChecker.IsIstioActive(ctx) {
		allowedPorts = append(allowedPorts, ports.IstioEnvoy)
	}
	networkPolicy := commonresources.MakeNetworkPolicy(r.config.DaemonSet, allowedPorts, fluentbit.Labels())
	if err := k8sutils.CreateOrUpdateNetworkPolicy(ctx, ownerRefSetter, networkPolicy); err != nil {
		return fmt.Errorf("failed to create fluent bit network policy: %w", err)
	}

	return nil
}

func (r *Reconciler) deleteResources(ctx context.Context) error {
	// Attempt to clean up as many resources as possible and avoid early return when one of the deletions fails
	var allErrors error = nil

	name := types.NamespacedName{Name: r.config.DaemonSet.Name, Namespace: r.config.DaemonSet.Namespace}

	objectMeta := metav1.ObjectMeta{
		Name:      name.Name,
		Namespace: name.Namespace,
	}

	serviceAccount := corev1.ServiceAccount{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, r.Client, &serviceAccount); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete serviceaccount: %w", err))
	}

	clusterRole := rbacv1.ClusterRole{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, r.Client, &clusterRole); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete clusterole: %w", err))
	}

	clusterRoleBinding := rbacv1.ClusterRoleBinding{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, r.Client, &clusterRoleBinding); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete clusterolebinding: %w", err))
	}

	exporterMetricsService := corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-exporter-metrics", name.Name), Namespace: name.Namespace}}
	if err := k8sutils.DeleteObject(ctx, r.Client, &exporterMetricsService); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete exporter metric service: %w", err))
	}

	metricsService := corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-metrics", name.Name), Namespace: name.Namespace}}
	if err := k8sutils.DeleteObject(ctx, r.Client, &metricsService); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete metric service: %w", err))
	}

	cm := corev1.ConfigMap{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, r.Client, &cm); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete configmap: %w", err))
	}

	luaCm := corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:      r.config.LuaConfigMap.Name,
		Namespace: r.config.LuaConfigMap.Namespace,
	}}
	if err := k8sutils.DeleteObject(ctx, r.Client, &luaCm); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete lua configmap: %w", err))
	}

	parserCm := corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:      r.config.ParsersConfigMap.Name,
		Namespace: r.config.ParsersConfigMap.Namespace,
	}}
	if err := k8sutils.DeleteObject(ctx, r.Client, &parserCm); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete parser configmap: %w", err))
	}

	sectionCm := corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:      r.config.SectionsConfigMap.Name,
		Namespace: r.config.SectionsConfigMap.Namespace,
	}}
	if err := k8sutils.DeleteObject(ctx, r.Client, &sectionCm); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete section configmap: %w", err))
	}

	filesCm := corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:      r.config.FilesConfigMap.Name,
		Namespace: r.config.FilesConfigMap.Namespace,
	}}
	if err := k8sutils.DeleteObject(ctx, r.Client, &filesCm); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete files configmap: %w", err))
	}

	daemonSet := appsv1.DaemonSet{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, r.Client, &daemonSet); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete daemonset: %w", err))
	}

	networkPolicy := networkingv1.NetworkPolicy{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, r.Client, &networkPolicy); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete networkpolicy: %w", err))
	}

	return allErrors
}

func (r *Reconciler) calculateChecksum(ctx context.Context) (string, error) {
	var baseCm corev1.ConfigMap
	if err := r.Get(ctx, r.config.DaemonSet, &baseCm); err != nil {
		return "", fmt.Errorf("failed to get %s/%s ConfigMap: %w", r.config.DaemonSet.Namespace, r.config.DaemonSet.Name, err)
	}

	var parsersCm corev1.ConfigMap
	if err := r.Get(ctx, r.config.ParsersConfigMap, &parsersCm); err != nil {
		return "", fmt.Errorf("failed to get %s/%s ConfigMap: %w", r.config.ParsersConfigMap.Namespace, r.config.ParsersConfigMap.Name, err)
	}

	var luaCm corev1.ConfigMap
	if err := r.Get(ctx, r.config.LuaConfigMap, &luaCm); err != nil {
		return "", fmt.Errorf("failed to get %s/%s ConfigMap: %w", r.config.LuaConfigMap.Namespace, r.config.LuaConfigMap.Name, err)
	}

	var sectionsCm corev1.ConfigMap
	if err := r.Get(ctx, r.config.SectionsConfigMap, &sectionsCm); err != nil {
		return "", fmt.Errorf("failed to get %s/%s ConfigMap: %w", r.config.SectionsConfigMap.Namespace, r.config.SectionsConfigMap.Name, err)
	}

	var filesCm corev1.ConfigMap
	if err := r.Get(ctx, r.config.FilesConfigMap, &filesCm); err != nil {
		return "", fmt.Errorf("failed to get %s/%s ConfigMap: %w", r.config.FilesConfigMap.Namespace, r.config.FilesConfigMap.Name, err)
	}

	var envSecret corev1.Secret
	if err := r.Get(ctx, r.config.EnvSecret, &envSecret); err != nil {
		return "", fmt.Errorf("failed to get %s/%s Secret: %w", r.config.EnvSecret.Namespace, r.config.EnvSecret.Name, err)
	}

	var tlsSecret corev1.Secret
	if err := r.Get(ctx, r.config.OutputTLSConfigSecret, &tlsSecret); err != nil {
		return "", fmt.Errorf("failed to get %s/%s Secret: %w", r.config.OutputTLSConfigSecret.Namespace, r.config.OutputTLSConfigSecret.Name, err)
	}

	return configchecksum.Calculate([]corev1.ConfigMap{baseCm, parsersCm, luaCm, sectionsCm, filesCm}, []corev1.Secret{envSecret, tlsSecret}), nil
}

// getReconcilablePipelines returns the list of log pipelines that are ready to be rendered into the Fluent Bit configuration.
// A pipeline is deployable if it is not being deleted, all secret references exist, and it doesn't have the legacy grafana-loki output defined.
func (r *Reconciler) getReconcilablePipelines(ctx context.Context, allPipelines []telemetryv1alpha1.LogPipeline) ([]telemetryv1alpha1.LogPipeline, error) {
	var reconcilableLogPipelines []telemetryv1alpha1.LogPipeline
	for i := range allPipelines {
		isReconcilable, err := r.isReconcilable(ctx, &allPipelines[i])
		if err != nil {
			return nil, err
		}
		if isReconcilable {
			reconcilableLogPipelines = append(reconcilableLogPipelines, allPipelines[i])
		}
	}

	return reconcilableLogPipelines, nil
}

func (r *Reconciler) isReconcilable(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) (bool, error) {
	if !pipeline.GetDeletionTimestamp().IsZero() {
		return false, nil
	}

	err := r.pipelineValidator.validate(ctx, pipeline)

	// Pipeline with a certificate that is about to expire is still considered reconcilable
	if err == nil || tlscert.IsCertAboutToExpireError(err) {
		return true, nil
	}

	// Remaining errors imply that the pipeline is not reconcilable
	// In case that one of the requests to the Kubernetes API server failed, then the pipeline is also considered non-reconcilable and the error is returned to trigger a requeue
	var APIRequestFailed *errortypes.APIRequestFailedError
	if errors.As(err, &APIRequestFailed) {
		return false, APIRequestFailed.Err
	}

	return false, nil
}

func getFluentBitPorts() []int32 {
	return []int32{
		ports.ExporterMetrics,
		ports.HTTP,
	}
}

func tlsValidationRequired(pipeline *telemetryv1alpha1.LogPipeline) bool {
	http := pipeline.Spec.Output.HTTP
	if http == nil {
		return false
	}
	return http.TLSConfig.Cert != nil || http.TLSConfig.Key != nil || http.TLSConfig.CA != nil
}
