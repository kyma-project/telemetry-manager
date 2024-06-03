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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/ports"
	"github.com/kyma-project/telemetry-manager/internal/istiostatus"
	"github.com/kyma-project/telemetry-manager/internal/k8sutils"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
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

//go:generate mockery --name DaemonSetProber --filename daemon_set_prober.go
type DaemonSetProber interface {
	IsReady(ctx context.Context, name types.NamespacedName) (bool, error)
}

//go:generate mockery --name DaemonSetAnnotator --filename daemon_set_annotator.go
type DaemonSetAnnotator interface {
	SetAnnotation(ctx context.Context, name types.NamespacedName, key, value string) error
}

//go:generate mockery --name TLSCertValidator --filename tls_cert_validator.go
type TLSCertValidator interface {
	ValidateCertificate(ctx context.Context, config tlscert.TLSConfig) error
}

//go:generate mockery --name FlowHealthProber --filename flow_health_prober.go
type FlowHealthProber interface {
	Probe(ctx context.Context, pipelineName string) (prober.LogPipelineProbeResult, error)
}

//go:generate mockery --name OverridesHandler --filename overrides_handler.go
type OverridesHandler interface {
	LoadOverrides(ctx context.Context) (*overrides.Config, error)
}

//go:generate mockery --name IstioStatusChecker --filename istio_status_checker.go
type IstioStatusChecker interface {
	IsIstioActive(ctx context.Context) bool
}

type Reconciler struct {
	client.Client
	config                     Config
	pipelinesConditionsCleared bool

	prober             DaemonSetProber
	flowHealthProber   FlowHealthProber
	tlsCertValidator   TLSCertValidator
	syncer             syncer
	overridesHandler   OverridesHandler
	istioStatusChecker IstioStatusChecker
}

func NewReconciler(
	client client.Client,
	config Config,
	agentProber DaemonSetProber,
	flowHealthProber FlowHealthProber,
	overridesHandler *overrides.Handler) *Reconciler {
	var r Reconciler
	r.Client = client
	r.config = config
	r.prober = agentProber
	r.flowHealthProber = flowHealthProber
	r.syncer = syncer{client, config}
	r.overridesHandler = overridesHandler
	r.istioStatusChecker = istiostatus.NewChecker(client)
	r.tlsCertValidator = tlscert.New(client)

	return &r
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

	if err = r.clearPipelinesConditions(ctx, allPipelines.Items); err != nil {
		return fmt.Errorf("failed to clear the conditions list for log pipelines: %w", err)
	}

	if err = ensureFinalizers(ctx, r.Client, pipeline); err != nil {
		return err
	}

	reconcilablePipelines := r.getReconcilablePipelines(ctx, allPipelines.Items)
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

	includeSections := true
	if len(pipelines) == 0 {
		includeSections = false
	}
	cm := fluentbit.MakeConfigMap(r.config.DaemonSet, includeSections)
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
func (r *Reconciler) getReconcilablePipelines(ctx context.Context, allPipelines []telemetryv1alpha1.LogPipeline) []telemetryv1alpha1.LogPipeline {
	var reconcilableLogPipelines []telemetryv1alpha1.LogPipeline
	for i := range allPipelines {
		isReconcilable := r.isReconcilable(ctx, &allPipelines[i])
		if isReconcilable {
			reconcilableLogPipelines = append(reconcilableLogPipelines, allPipelines[i])
		}
	}

	return reconcilableLogPipelines
}

func (r *Reconciler) isReconcilable(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) bool {
	if !pipeline.GetDeletionTimestamp().IsZero() {
		return false
	}
	if secretref.ReferencesNonExistentSecret(ctx, r.Client, pipeline) {
		return false
	}
	if pipeline.Spec.Output.IsLokiDefined() {
		return false
	}

	if tlsCertValidationRequired(pipeline) {
		tlsConfig := tlscert.TLSConfig{
			Cert: pipeline.Spec.Output.HTTP.TLSConfig.Cert,
			Key:  pipeline.Spec.Output.HTTP.TLSConfig.Key,
			CA:   pipeline.Spec.Output.HTTP.TLSConfig.CA,
		}

		if err := r.tlsCertValidator.ValidateCertificate(ctx, tlsConfig); err != nil {
			if !tlscert.IsCertAboutToExpireError(err) {
				return false
			}
		}
	}

	return true
}

func getFluentBitPorts() []int32 {
	return []int32{
		ports.ExporterMetrics,
		ports.HTTP,
	}
}

func tlsCertValidationRequired(pipeline *telemetryv1alpha1.LogPipeline) bool {
	http := pipeline.Spec.Output.HTTP
	if http == nil {
		return false
	}
	return http.TLSConfig.Cert != nil || http.TLSConfig.Key != nil
}

// clearPipelinesConditions clears the status conditions for all LogPipelines only in the 1st reconciliation
// This is done to allow the legacy conditions ("Running" and "Pending") to be always appended at the end of the conditions list even if new condition types are added
// Check https://github.com/kyma-project/telemetry-manager/blob/main/docs/contributor/arch/004-consolidate-pipeline-statuses.md#decision
// TODO: Remove this logic after the end of the deprecation period of the legacy conditions ("Running" and "Pending")
func (r *Reconciler) clearPipelinesConditions(ctx context.Context, allPipelines []telemetryv1alpha1.LogPipeline) error {
	if r.pipelinesConditionsCleared {
		return nil
	}

	for i := range allPipelines {
		allPipelines[i].Status.Conditions = []metav1.Condition{}
		if err := r.Status().Update(ctx, &allPipelines[i]); err != nil {
			return fmt.Errorf("failed to update LogPipeline status: %w", err)
		}
	}
	r.pipelinesConditionsCleared = true

	return nil
}
