package fluentbit

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
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/ports"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

type Config struct {
	DaemonSet           types.NamespacedName
	SectionsConfigMap   types.NamespacedName
	FilesConfigMap      types.NamespacedName
	LuaConfigMap        types.NamespacedName
	ParsersConfigMap    types.NamespacedName
	EnvConfigSecret     types.NamespacedName
	TLSFileConfigSecret types.NamespacedName
	PipelineDefaults    builder.PipelineDefaults
	Overrides           overrides.Config
	DaemonSetConfig     fluentbit.DaemonSetConfig
	RestConfig          rest.Config
}

var _ logpipeline.LogPipelineReconciler = &Reconciler{}

type Reconciler struct {
	client.Client

	config Config
	syncer syncer

	// Dependencies
	agentProber        commonstatus.Prober
	flowHealthProber   logpipeline.FlowHealthProber
	istioStatusChecker logpipeline.IstioStatusChecker
	pipelineValidator  *Validator
	errToMsgConverter  commonstatus.ErrorToMessageConverter
}

func (r *Reconciler) SupportedOutput() logpipelineutils.Mode {
	return logpipelineutils.FluentBit
}

func New(client client.Client, config Config, prober commonstatus.Prober, healthProber logpipeline.FlowHealthProber, checker logpipeline.IstioStatusChecker, validator *Validator, converter commonstatus.ErrorToMessageConverter) *Reconciler {
	return &Reconciler{
		Client:             client,
		config:             config,
		agentProber:        prober,
		flowHealthProber:   healthProber,
		istioStatusChecker: checker,
		pipelineValidator:  validator,
		errToMsgConverter:  converter,
		syncer: syncer{
			Client: client,
			config: config,
		},
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	logf.FromContext(ctx).V(1).Info("Reconciling LogPipeline")

	err := r.doReconcile(ctx, pipeline)

	if statusErr := r.updateStatus(ctx, pipeline.Name); statusErr != nil {
		if err != nil {
			err = fmt.Errorf("failed while updating status: %w: %w", statusErr, err)
		} else {
			err = fmt.Errorf("failed to update status: %w", statusErr)
		}
	}

	return err
}

func (r *Reconciler) doReconcile(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	allPipelines, err := logpipeline.GetPipelinesForType(ctx, r.Client, r.SupportedOutput())
	if err != nil {
		return err
	}

	err = ensureFinalizers(ctx, r.Client, pipeline)
	if err != nil {
		return err
	}

	reconcilablePipelines, err := r.getReconcilablePipelines(ctx, allPipelines)
	if err != nil {
		return fmt.Errorf("failed to fetch reconcilable log pipelines: %w", err)
	}

	if len(reconcilablePipelines) == 0 {
		logf.FromContext(ctx).V(1).Info("cleaning up log pipeline resources: all log pipelines are non-reconcilable")

		if err = r.deleteFluentBitResources(ctx); err != nil {
			return fmt.Errorf("failed to delete log pipeline resources: %w", err)
		}
	}

	if err = r.syncer.syncFluentBitConfig(ctx, pipeline, reconcilablePipelines); err != nil {
		return err
	}

	if err = r.createOrUpdateFluentBitResources(ctx, pipeline, reconcilablePipelines); err != nil {
		return err
	}

	if err = cleanupFinalizersIfNeeded(ctx, r.Client, pipeline); err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) createOrUpdateFluentBitResources(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline, pipelines []telemetryv1alpha1.LogPipeline) error {
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

func (r *Reconciler) deleteFluentBitResources(ctx context.Context) error {
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
	if err := r.Get(ctx, r.config.EnvConfigSecret, &envSecret); err != nil {
		return "", fmt.Errorf("failed to get %s/%s Secret: %w", r.config.EnvConfigSecret.Namespace, r.config.EnvConfigSecret.Name, err)
	}

	var tlsSecret corev1.Secret
	if err := r.Get(ctx, r.config.TLSFileConfigSecret, &tlsSecret); err != nil {
		return "", fmt.Errorf("failed to get %s/%s Secret: %w", r.config.TLSFileConfigSecret.Namespace, r.config.TLSFileConfigSecret.Name, err)
	}

	return configchecksum.Calculate([]corev1.ConfigMap{baseCm, parsersCm, luaCm, sectionsCm, filesCm}, []corev1.Secret{envSecret, tlsSecret}), nil
}

// getReconcilablePipelines returns the list of log pipelines that are ready to be rendered into the Fluent Bit configuration.
// A pipeline is deployable if it is not being deleted, and all secret references exist.
func (r *Reconciler) getReconcilablePipelines(ctx context.Context, allPipelines []telemetryv1alpha1.LogPipeline) ([]telemetryv1alpha1.LogPipeline, error) {
	var reconcilableLogPipelines []telemetryv1alpha1.LogPipeline

	for i := range allPipelines {
		isReconcilable, err := r.IsReconcilable(ctx, &allPipelines[i])
		if err != nil {
			return nil, err
		}

		if isReconcilable {
			reconcilableLogPipelines = append(reconcilableLogPipelines, allPipelines[i])
		}
	}

	return reconcilableLogPipelines, nil
}

func (r *Reconciler) IsReconcilable(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) (bool, error) {
	if !pipeline.GetDeletionTimestamp().IsZero() {
		return false, nil
	}

	var appInputEnabled *bool

	// Treat the pipeline as non-reconcilable if the application input is explicitly disabled
	if pipeline.Spec.Input.Application != nil {
		appInputEnabled = pipeline.Spec.Input.Application.Enabled
	}

	if appInputEnabled != nil && !*appInputEnabled {
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
