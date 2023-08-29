package metricpipeline

import (
	"context"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/kubernetes"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric/agent"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric/gateway"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	otelagentresources "github.com/kyma-project/telemetry-manager/internal/resources/otelcollector/agent"
	otelcoreresources "github.com/kyma-project/telemetry-manager/internal/resources/otelcollector/core"
	otelgatewayresources "github.com/kyma-project/telemetry-manager/internal/resources/otelcollector/gateway"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
)

type Config struct {
	Agent                  otelagentresources.Config
	Gateway                otelgatewayresources.Config
	OverridesConfigMapName types.NamespacedName
	MaxPipelines           int
}

//go:generate mockery --name DeploymentProber --filename deployment_prober.go
type DeploymentProber interface {
	IsReady(ctx context.Context, name types.NamespacedName) (bool, error)
}

type Reconciler struct {
	client.Client
	config           Config
	prober           DeploymentProber
	overridesHandler overrides.GlobalConfigHandler
}

func NewReconciler(client client.Client, config Config, prober DeploymentProber, overridesHandler overrides.GlobalConfigHandler) *Reconciler {
	return &Reconciler{
		Client:           client,
		config:           config,
		prober:           prober,
		overridesHandler: overridesHandler,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	log.V(1).Info("Reconciliation triggered")

	overrideConfig, err := r.overridesHandler.UpdateOverrideConfig(ctx, r.config.OverridesConfigMapName)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.overridesHandler.CheckGlobalConfig(overrideConfig.Global); err != nil {
		return ctrl.Result{}, err
	}
	if overrideConfig.Metrics.Paused {
		log.V(1).Info("Skipping reconciliation of metricpipeline as reconciliation is paused")
		return ctrl.Result{}, nil
	}

	var metricPipeline telemetryv1alpha1.MetricPipeline
	if err := r.Get(ctx, req.NamespacedName, &metricPipeline); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return ctrl.Result{}, r.doReconcile(ctx, &metricPipeline)
}

func (r *Reconciler) doReconcile(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline) error {
	var err error
	lockAcquired := true

	defer func() {
		if statusErr := r.updateStatus(ctx, pipeline.Name, lockAcquired); statusErr != nil {
			if err != nil {
				err = fmt.Errorf("failed while updating status: %v: %v", statusErr, err)
			} else {
				err = fmt.Errorf("failed to update status: %v", statusErr)
			}
		}
	}()

	lock := kubernetes.NewResourceCountLock(r.Client, types.NamespacedName{
		Name:      "telemetry-metricpipeline-lock",
		Namespace: r.config.Gateway.Namespace,
	}, r.config.MaxPipelines)
	if err = lock.TryAcquireLock(ctx, pipeline); err != nil {
		lockAcquired = false
		return err
	}

	var allPipelinesList telemetryv1alpha1.MetricPipelineList
	if err = r.List(ctx, &allPipelinesList); err != nil {
		return fmt.Errorf("failed to list metric pipelines: %w", err)
	}
	deployablePipelines, err := getDeployableMetricPipelines(ctx, allPipelinesList.Items, r, lock)
	if err != nil {
		return fmt.Errorf("failed to fetch deployable metric pipelines: %w", err)
	}
	if len(deployablePipelines) == 0 {
		return fmt.Errorf("no metric pipeline ready for deployment")
	}

	if err = r.reconcileMetricGateway(ctx, pipeline, deployablePipelines); err != nil {
		return fmt.Errorf("failed to reconcile metric gateway: %w", err)
	}

	if isMetricAgentRequired(pipeline) {
		if err = r.reconcileMetricAgents(ctx, pipeline, allPipelinesList.Items); err != nil {
			return fmt.Errorf("failed to reconcile metric agents: %w", err)
		}
	}

	return nil
}

// getDeployableMetricPipelines returns the list of metric pipelines that are ready to be rendered into the otel collector configuration. A pipeline is deployable if it is not being deleted, all secret references exist, and is not above the pipeline limit.
func getDeployableMetricPipelines(ctx context.Context, allPipelines []telemetryv1alpha1.MetricPipeline, client client.Client, lock *kubernetes.ResourceCountLock) ([]telemetryv1alpha1.MetricPipeline, error) {
	var deployablePipelines []telemetryv1alpha1.MetricPipeline
	for i := range allPipelines {
		if !allPipelines[i].GetDeletionTimestamp().IsZero() {
			continue
		}

		if secretref.ReferencesNonExistentSecret(ctx, client, &allPipelines[i]) {
			continue
		}

		hasLock, err := lock.IsLockHolder(ctx, &allPipelines[i])
		if err != nil {
			return nil, err
		}

		if hasLock {
			deployablePipelines = append(deployablePipelines, allPipelines[i])
		}
	}
	return deployablePipelines, nil
}

func (r *Reconciler) reconcileMetricGateway(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline, allPipelines []telemetryv1alpha1.MetricPipeline) error {
	namespacedBaseName := types.NamespacedName{
		Name:      r.config.Gateway.BaseName,
		Namespace: r.config.Gateway.Namespace,
	}

	var err error
	serviceAccount := commonresources.MakeServiceAccount(namespacedBaseName)
	if err = controllerutil.SetOwnerReference(pipeline, serviceAccount, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateServiceAccount(ctx, r, serviceAccount); err != nil {
		return fmt.Errorf("failed to create otel collector service account: %w", err)
	}

	clusterRole := otelgatewayresources.MakeClusterRole(namespacedBaseName)
	if err = controllerutil.SetOwnerReference(pipeline, clusterRole, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateClusterRole(ctx, r, clusterRole); err != nil {
		return fmt.Errorf("failed to create otel collector cluster role: %w", err)
	}

	clusterRoleBinding := commonresources.MakeClusterRoleBinding(namespacedBaseName)
	if err = controllerutil.SetOwnerReference(pipeline, clusterRoleBinding, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateClusterRoleBinding(ctx, r, clusterRoleBinding); err != nil {
		return fmt.Errorf("failed to create otel collector cluster role Binding: %w", err)
	}

	gatewayConfig, envVars, err := gateway.MakeConfig(ctx, r, allPipelines)
	if err != nil {
		return fmt.Errorf("failed to make otel collector config: %v", err)
	}

	var gatewayConfigYAML []byte
	gatewayConfigYAML, err = yaml.Marshal(gatewayConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal collector config: %w", err)
	}

	secret := otelgatewayresources.MakeSecret(r.config.Gateway, envVars)
	if err = controllerutil.SetOwnerReference(pipeline, secret, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateSecret(ctx, r.Client, secret); err != nil {
		return fmt.Errorf("failed to create otel collector env secret: %w", err)
	}

	configMap := otelcoreresources.MakeConfigMap(namespacedBaseName, string(gatewayConfigYAML))
	if err = controllerutil.SetOwnerReference(pipeline, configMap, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateConfigMap(ctx, r.Client, configMap); err != nil {
		return fmt.Errorf("failed to create otel collector configmap: %w", err)
	}

	configHash := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, []corev1.Secret{*secret})
	deployment := otelgatewayresources.MakeDeployment(r.config.Gateway, configHash, len(allPipelines),
		config.EnvVarCurrentPodIP, config.EnvVarCurrentNodeName)
	if err = controllerutil.SetOwnerReference(pipeline, deployment, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateDeployment(ctx, r.Client, deployment); err != nil {
		return fmt.Errorf("failed to create otel collector deployment: %w", err)
	}

	otlpService := otelgatewayresources.MakeOTLPService(r.config.Gateway)
	if err = controllerutil.SetOwnerReference(pipeline, otlpService, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateService(ctx, r.Client, otlpService); err != nil {
		//nolint:dupword // otel collector collector service is a real name.
		return fmt.Errorf("failed to create otel collector collector service: %w", err)
	}

	metricsService := otelgatewayresources.MakeMetricsService(r.config.Gateway)
	if err = controllerutil.SetOwnerReference(pipeline, metricsService, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateService(ctx, r.Client, metricsService); err != nil {
		return fmt.Errorf("failed to create otel collector metrics service: %w", err)
	}

	networkPolicyPorts := makeNetworkPolicyPorts()
	networkPolicy := otelgatewayresources.MakeNetworkPolicy(r.config.Gateway, networkPolicyPorts)
	if err = controllerutil.SetOwnerReference(pipeline, networkPolicy, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateNetworkPolicy(ctx, r.Client, networkPolicy); err != nil {
		return fmt.Errorf("failed to create otel collector network policy: %w", err)
	}

	return nil
}

func makeNetworkPolicyPorts() []intstr.IntOrString {
	return []intstr.IntOrString{
		intstr.FromInt(ports.OTLPHTTP),
		intstr.FromInt(ports.OTLPGRPC),
		intstr.FromInt(ports.OpenCensus),
		intstr.FromInt(ports.Metrics),
		intstr.FromInt(ports.HealthCheck),
	}
}

func isMetricAgentRequired(pipeline *telemetryv1alpha1.MetricPipeline) bool {
	return pipeline.Spec.Input.Application.Runtime.Enabled || pipeline.Spec.Input.Application.Prometheus.Enabled || pipeline.Spec.Input.Application.Istio.Enabled
}

func (r *Reconciler) isIstioDeployed(ctx context.Context) bool {
	var crdList apiextensionsv1.CustomResourceDefinitionList
	if err := r.List(ctx, &crdList); err != nil {
		logf.FromContext(ctx).Error(err, "Not able to list CRDs")
		//no kind found
		if _, ok := err.(*meta.NoKindMatchError); ok {
			return false
		}
		return false
	}

	var names []string
	for _, crd := range crdList.Items {
		names = append(names, crd.Name)
	}

	logf.FromContext(ctx).Info("Found CRDs", "crds", names)

	for _, crd := range crdList.Items {
		if strings.EqualFold(crd.Name, "peerauthentications.security.istio.io") {
			return true
		}
	}

	return false
}

func (r *Reconciler) reconcileMetricAgents(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline, allPipelines []telemetryv1alpha1.MetricPipeline) error {
	namespacedBaseName := types.NamespacedName{
		Name:      r.config.Agent.BaseName,
		Namespace: r.config.Agent.Namespace,
	}

	var err error
	serviceAccount := commonresources.MakeServiceAccount(namespacedBaseName)
	if err = controllerutil.SetOwnerReference(pipeline, serviceAccount, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateServiceAccount(ctx, r, serviceAccount); err != nil {
		return fmt.Errorf("failed to create otel collector service account: %w", err)
	}

	clusterRole := otelagentresources.MakeClusterRole(namespacedBaseName)
	if err = controllerutil.SetOwnerReference(pipeline, clusterRole, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateClusterRole(ctx, r, clusterRole); err != nil {
		return fmt.Errorf("failed to create otel collector cluster role: %w", err)
	}

	clusterRoleBinding := commonresources.MakeClusterRoleBinding(namespacedBaseName)
	if err = controllerutil.SetOwnerReference(pipeline, clusterRoleBinding, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateClusterRoleBinding(ctx, r, clusterRoleBinding); err != nil {
		return fmt.Errorf("failed to create otel collector cluster role Binding: %w", err)
	}

	agentConfig := agent.MakeConfig(types.NamespacedName{
		Namespace: r.config.Gateway.Namespace,
		Name:      r.config.Gateway.Service.OTLPServiceName,
	}, allPipelines, r.isIstioDeployed(ctx))
	var agentConfigYAML []byte
	agentConfigYAML, err = yaml.Marshal(agentConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal collector config: %w", err)
	}

	configMap := otelcoreresources.MakeConfigMap(namespacedBaseName, string(agentConfigYAML))
	if err = controllerutil.SetOwnerReference(pipeline, configMap, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateConfigMap(ctx, r.Client, configMap); err != nil {
		return fmt.Errorf("failed to create otel collector configmap: %w", err)
	}

	configHash := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, []corev1.Secret{})
	daemonSet := otelagentresources.MakeDaemonSet(r.config.Agent, configHash, config.EnvVarCurrentPodIP, config.EnvVarCurrentNodeName, agent.IstioCertPath)
	if err = controllerutil.SetOwnerReference(pipeline, daemonSet, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateDaemonSet(ctx, r.Client, daemonSet); err != nil {
		return fmt.Errorf("failed to create otel collector deployment: %w", err)
	}

	return nil
}
