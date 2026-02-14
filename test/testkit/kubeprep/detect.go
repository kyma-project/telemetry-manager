package kubeprep

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
//   - FIPS Mode: Check telemetry-manager deployment env var OPERATE_IN_FIPS_MODE
//   - Experimental: Check for marker label on telemetry-manager deployment
//   - Manager: Check if telemetry-manager deployment exists
//
// If the manager is deployed but missing the experimental label, NeedsReinstall is set to true
// because we cannot reliably determine the current configuration.
//
// Returns a Config representing the detected state, or error if detection fails.
func DetectClusterState(t TestingT, k8sClient client.Client) (*Config, error) {
	t.Helper()
	ctx := t.Context()

	managerDeployed := detectManagerDeployed(t, k8sClient)
	experimentalEnabled, hasLabel := detectExperimentalEnabled(t, k8sClient)

	// If manager is deployed but label is missing, we need to reinstall to get a known state
	needsReinstall := managerDeployed && !hasLabel

	cfg := &Config{
		// ManagerImage and LocalImage cannot be reliably detected, leave empty
		// These will be populated from environment or defaults when needed
		ManagerImage: "",
		LocalImage:   false,

		// Detect actual cluster state
		InstallIstio:            detectIstioInstalled(t, k8sClient),
		OperateInFIPSMode:       detectFIPSMode(t, k8sClient),
		EnableExperimental:      experimentalEnabled,
		CustomLabelsAnnotations: false, // Cannot reliably detect, assume false
		SkipManagerDeployment:   !managerDeployed,
		SkipPrerequisites:       false, // Cannot reliably detect, assume false
		NeedsReinstall:          needsReinstall,
	}

	t.Logf("Detected cluster state: InstallIstio=%t, OperateInFIPSMode=%t, EnableExperimental=%t, ManagerDeployed=%t, NeedsReinstall=%t",
		cfg.InstallIstio, cfg.OperateInFIPSMode, cfg.EnableExperimental, managerDeployed, cfg.NeedsReinstall)

	// Store context for later use by other functions
	_ = ctx

	return cfg, nil
}

// detectIstioInstalled checks if Istio is installed by looking for the default Istio CR
func detectIstioInstalled(t TestingT, k8sClient client.Client) bool {
	t.Helper()
	ctx := t.Context()

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

	if err == nil {
		t.Log("Detected: Istio is installed (default Istio CR found)")
		return true
	}

	if apierrors.IsNotFound(err) {
		t.Log("Detected: Istio is not installed (default Istio CR not found)")
		return false
	}

	// If we can't determine (e.g., CRD doesn't exist), assume not installed
	t.Logf("Warning: Could not detect Istio state: %v (assuming not installed)", err)
	return false
}

// detectFIPSMode checks if telemetry manager is running in FIPS mode
func detectFIPSMode(t TestingT, k8sClient client.Client) bool {
	t.Helper()
	ctx := t.Context()

	deployment := &appsv1.Deployment{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      telemetryManagerName,
		Namespace: kymaSystemNamespace,
	}, deployment)

	if err != nil {
		if apierrors.IsNotFound(err) {
			t.Log("Detected: Manager not deployed, cannot detect FIPS mode (assuming false)")
			return false
		}
		t.Logf("Warning: Could not get manager deployment: %v (assuming FIPS=false)", err)
		return false
	}

	// Check for OPERATE_IN_FIPS_MODE env var in manager container
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == "manager" {
			for _, env := range container.Env {
				if env.Name == "OPERATE_IN_FIPS_MODE" {
					fipsEnabled := env.Value == "true"
					t.Logf("Detected: FIPS mode = %t (from manager deployment env)", fipsEnabled)
					return fipsEnabled
				}
			}
		}
	}

	// If env var not found, assume false (default)
	t.Log("Detected: FIPS mode = false (OPERATE_IN_FIPS_MODE env var not found)")
	return false
}

// detectExperimentalEnabled checks if experimental features are enabled by looking for the marker label on the manager deployment.
// Returns (experimentalEnabled, labelFound) - if label is not found, the state is unknown.
func detectExperimentalEnabled(t TestingT, k8sClient client.Client) (bool, bool) {
	t.Helper()
	ctx := t.Context()

	deployment := &appsv1.Deployment{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      telemetryManagerName,
		Namespace: kymaSystemNamespace,
	}, deployment)

	if err != nil {
		if apierrors.IsNotFound(err) {
			t.Log("Detected: Manager not deployed, cannot detect experimental mode")
			return false, false
		}
		t.Logf("Warning: Could not get manager deployment: %v", err)
		return false, false
	}

	// Check for the experimental-enabled label on the deployment
	if deployment.Labels != nil {
		if value, exists := deployment.Labels[LabelExperimentalEnabled]; exists {
			experimentalEnabled := value == "true"
			t.Logf("Detected: Experimental mode = %t (from manager deployment label)", experimentalEnabled)
			return experimentalEnabled, true
		}
	}

	// Label not found - state is unknown
	t.Log("Detected: Experimental label not found on manager deployment - state unknown, reinstall required")
	return false, false
}

// detectManagerDeployed checks if telemetry manager is deployed
func detectManagerDeployed(t TestingT, k8sClient client.Client) bool {
	t.Helper()
	ctx := t.Context()

	deployment := &appsv1.Deployment{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      telemetryManagerName,
		Namespace: kymaSystemNamespace,
	}, deployment)

	if err == nil {
		t.Log("Detected: Telemetry manager is deployed")
		return true
	}

	if apierrors.IsNotFound(err) {
		t.Log("Detected: Telemetry manager is not deployed")
		return false
	}

	// If we can't determine, assume not deployed
	t.Logf("Warning: Could not detect manager deployment: %v (assuming not deployed)", err)
	return false
}

// DetectOrUseProvidedConfig returns the provided config if available, otherwise detects cluster state.
// This is the recommended function to use for initializing CurrentClusterState.
func DetectOrUseProvidedConfig(t TestingT, k8sClient client.Client, providedConfig *Config) (*Config, error) {
	t.Helper()

	if providedConfig != nil {
		t.Log("Using provided cluster configuration")
		return providedConfig, nil
	}

	t.Log("No cluster configuration provided, detecting current state...")
	detectedConfig, err := DetectClusterState(t, k8sClient)
	if err != nil {
		return nil, fmt.Errorf("failed to detect cluster state: %w", err)
	}

	return detectedConfig, nil
}
