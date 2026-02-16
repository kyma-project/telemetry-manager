package kubeprep

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DetectClusterState inspects the cluster to determine its current configuration.
// This allows the system to work with existing clusters without requiring ClusterPrepConfig.
//
// Detection logic:
//   - Istio: Check if default Istio CR exists (istios.operator.kyma-project.io/default in kyma-system)
//   - FIPS Mode: Check telemetry-manager deployment env var KYMA_FIPS_MODE_ENABLED
//   - Experimental: Check for --deploy-otlp-gateway=true arg on telemetry-manager deployment
//   - Manager: Check if telemetry-manager deployment exists
//   - Helm Customized: Check for helm-customized label on deployment
//
// Returns a Config representing the detected state, or error if detection fails.
// If k8sClient is nil, returns a default config (empty cluster state).
func DetectClusterState(t TestingT, k8sClient client.Client) (*Config, error) {
	// Handle nil client gracefully - return default config
	if k8sClient == nil {
		t.Logf("No k8s client available, returning default cluster state")
		return &Config{
			ManagerImage:          "",
			LocalImage:            false,
			InstallIstio:          false,
			OperateInFIPSMode:     false,
			EnableExperimental:    false,
			SkipManagerDeployment: true,
			SkipPrerequisites:     false,
		}, nil
	}

	ctx := t.Context()

	managerDeployed := detectManagerDeployed(ctx, k8sClient)

	cfg := &Config{
		// ManagerImage and LocalImage cannot be reliably detected, leave empty
		// These will be populated from environment or defaults when needed
		ManagerImage: "",
		LocalImage:   false,

		// Detect actual cluster state
		InstallIstio:          detectIstioInstalled(ctx, k8sClient),
		OperateInFIPSMode:     detectFIPSMode(ctx, k8sClient),
		EnableExperimental:    detectExperimentalEnabled(ctx, k8sClient),
		SkipManagerDeployment: !managerDeployed,
		SkipPrerequisites:     false, // Cannot reliably detect, assume false
	}

	// If deployment has customization marker, set NeedsReinstall to force clean state
	if detectHelmCustomized(ctx, k8sClient) {
		cfg.NeedsReinstall = true
		t.Log("Detected customized manager deployment - will reinstall to restore clean state")
	}

	t.Logf("Detected: istio=%t, fips=%t, experimental=%t, manager=%t, customized=%t",
		cfg.InstallIstio, cfg.OperateInFIPSMode, cfg.EnableExperimental, managerDeployed, cfg.NeedsReinstall)

	return cfg, nil
}

// detectIstioInstalled checks if Istio is installed by looking for the default Istio CR
func detectIstioInstalled(ctx context.Context, k8sClient client.Client) bool {
	// Check if the default Istio CR exists in kyma-system namespace
	// This is the CR created by our Istio installation: istios.operator.kyma-project.io/default
	istioCR := &unstructured.Unstructured{}
	istioCR.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.kyma-project.io",
		Version: "v1alpha2",
		Kind:    "Istio",
	})

	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      "default",
		Namespace: "kyma-system",
	}, istioCR)

	return err == nil
}

// detectFIPSMode checks if telemetry manager is running in FIPS mode
func detectFIPSMode(ctx context.Context, k8sClient client.Client) bool {
	deployment := &appsv1.Deployment{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      telemetryManagerName,
		Namespace: kymaSystemNamespace,
	}, deployment)

	if err != nil {
		return false
	}

	// Check for KYMA_FIPS_MODE_ENABLED env var in manager container
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == "manager" {
			for _, env := range container.Env {
				if env.Name == "KYMA_FIPS_MODE_ENABLED" {
					return env.Value == "true"
				}
			}
		}
	}

	return false
}

// detectExperimentalEnabled checks if experimental features are enabled
// by looking for the custom label on the manager deployment.
func detectExperimentalEnabled(ctx context.Context, k8sClient client.Client) bool {
	deployment := &appsv1.Deployment{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      telemetryManagerName,
		Namespace: kymaSystemNamespace,
	}, deployment)

	if err != nil {
		return false
	}

	// Check for the experimental-enabled label on the deployment
	if deployment.Labels != nil {
		if value, exists := deployment.Labels[LabelExperimentalEnabled]; exists {
			return value == "true"
		}
	}

	return false
}

// detectManagerDeployed checks if telemetry manager is deployed
func detectManagerDeployed(ctx context.Context, k8sClient client.Client) bool {
	deployment := &appsv1.Deployment{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      telemetryManagerName,
		Namespace: kymaSystemNamespace,
	}, deployment)

	return err == nil
}

// detectHelmCustomized checks if manager was deployed with custom helm values
func detectHelmCustomized(ctx context.Context, k8sClient client.Client) bool {
	deployment := &appsv1.Deployment{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      telemetryManagerName,
		Namespace: kymaSystemNamespace,
	}, deployment)

	if err != nil {
		return false
	}

	if deployment.Labels != nil {
		if value, exists := deployment.Labels[LabelHelmCustomized]; exists {
			return value == "true"
		}
	}

	return false
}

// DetectOrUseProvidedConfig returns the provided config if available, otherwise detects cluster state.
// This is the recommended function to use for initializing CurrentClusterState.
func DetectOrUseProvidedConfig(t TestingT, k8sClient client.Client, providedConfig *Config) (*Config, error) {
	if providedConfig != nil {
		return providedConfig, nil
	}

	return DetectClusterState(t, k8sClient)
}
