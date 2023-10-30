package otelcollector

import (
	"context"
	"fmt"
	"maps"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/kubernetes"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
)

func ApplyGatewayResources(ctx context.Context, c client.Client, cfg *GatewayConfig) error {
	if err := applyCommonResources(ctx, c, cfg); err != nil {
		return fmt.Errorf("failed to apply common otel collector resource: %w", err)
	}

	return nil
}

func applyCommonResources(ctx context.Context, c client.Client, cfg *GatewayConfig) error {
	var err error
	name := types.NamespacedName{Namespace: cfg.Namespace, Name: cfg.BaseName}

	serviceAccount := commonresources.MakeServiceAccount(name)
	if err = kubernetes.CreateOrUpdateServiceAccount(ctx, c, serviceAccount); err != nil {
		return fmt.Errorf("failed to create serviceaccount: %w", err)
	}

	clusterRole := makeGatewayClusterRole(name)
	if err = kubernetes.CreateOrUpdateClusterRole(ctx, c, clusterRole); err != nil {
		return fmt.Errorf("failed to create clusterrole: %w", err)
	}

	clusterRoleBinding := commonresources.MakeClusterRoleBinding(name)
	if err = kubernetes.CreateOrUpdateClusterRoleBinding(ctx, c, clusterRoleBinding); err != nil {
		return fmt.Errorf("failed to create clusterrolebinding: %w", err)
	}

	secret := makeSecret(name, cfg.CollectorEnvVars)
	if err = kubernetes.CreateOrUpdateSecret(ctx, c, secret); err != nil {
		return fmt.Errorf("failed to create env secret: %w", err)
	}

	configMap := makeConfigMap(name, cfg.CollectorConfig)
	if err = kubernetes.CreateOrUpdateConfigMap(ctx, c, configMap); err != nil {
		return fmt.Errorf("failed to create configmap: %w", err)
	}

	configChecksum := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, []corev1.Secret{*secret})
	deployment := makeGatewayDeployment(cfg, configChecksum)
	if err = kubernetes.CreateOrUpdateDeployment(ctx, c, deployment); err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	otlpService := makeOTLPService(cfg)
	if err = kubernetes.CreateOrUpdateService(ctx, c, otlpService); err != nil {
		return fmt.Errorf("failed to create otel collector otlp service: %w", err)
	}

	metricsService := makeMetricsService(&cfg.Config)
	if err = kubernetes.CreateOrUpdateService(ctx, c, metricsService); err != nil {
		return fmt.Errorf("failed to create otel collector metrics service: %w", err)
	}

	networkPolicy := makeNetworkPolicy(&cfg.Config)
	if err = kubernetes.CreateOrUpdateNetworkPolicy(ctx, c, networkPolicy); err != nil {
		return fmt.Errorf("failed to create otel collector network policy: %w", err)
	}

	if cfg.CanReceiveOpenCensus {
		openCensusService := makeOpenCensusService(&cfg.Config)
		if err = kubernetes.CreateOrUpdateService(ctx, c, openCensusService); err != nil {
			return fmt.Errorf("failed to create otel collector metrics service: %w", err)
		}
	}

	return nil
}

func makeGatewayClusterRole(name types.NamespacedName) *rbacv1.ClusterRole {
	clusterRole := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces", "pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"replicasets"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
	return &clusterRole
}

func makeGatewayDeployment(cfg *GatewayConfig, configChecksum string) *appsv1.Deployment {
	selectorLabels := defaultLabels(cfg.BaseName)
	podLabels := maps.Clone(selectorLabels)
	podLabels["sidecar.istio.io/inject"] = "false"

	annotations := makeCommonPodAnnotations(configChecksum)
	resources := makeGatewayResourceRequirements(cfg)
	affinity := makePodAffinity(selectorLabels)
	podSpec := makePodSpec(cfg.BaseName, cfg.Deployment.Image,
		withPriorityClass(cfg.Deployment.PriorityClassName),
		withResources(resources),
		withAffinity(affinity),
		withEnvVarFromSource(config.EnvVarCurrentPodIP, fieldPathPodIP),
		withEnvVarFromSource(config.EnvVarCurrentNodeName, fieldPathNodeName),
	)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.BaseName,
			Namespace: cfg.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(cfg.Scaling.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      podLabels,
					Annotations: annotations,
				},
				Spec: podSpec,
			},
		},
	}
}

func makeGatewayResourceRequirements(cfg *GatewayConfig) corev1.ResourceRequirements {
	memoryRequest := cfg.Deployment.BaseMemoryRequest.DeepCopy()
	memoryLimit := cfg.Deployment.BaseMemoryLimit.DeepCopy()
	cpuRequest := cfg.Deployment.BaseCPURequest.DeepCopy()
	cpuLimit := cfg.Deployment.BaseCPULimit.DeepCopy()

	for i := 0; i < cfg.Scaling.ResourceRequirementsMultiplier; i++ {
		memoryRequest.Add(cfg.Deployment.DynamicMemoryRequest)
		memoryLimit.Add(cfg.Deployment.DynamicMemoryLimit)
		cpuRequest.Add(cfg.Deployment.DynamicCPURequest)
		cpuLimit.Add(cfg.Deployment.DynamicCPULimit)
	}

	resources := corev1.ResourceRequirements{
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    cpuRequest,
			corev1.ResourceMemory: memoryRequest,
		},
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    cpuLimit,
			corev1.ResourceMemory: memoryLimit,
		},
	}

	return resources
}

func makePodAffinity(labels map[string]string) corev1.Affinity {
	return corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						TopologyKey: "kubernetes.io/hostname",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: labels,
						},
					},
				},
				{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						TopologyKey: "topology.kubernetes.io/zone",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: labels,
						},
					},
				},
			},
		},
	}
}
