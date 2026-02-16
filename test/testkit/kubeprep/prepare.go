package kubeprep

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PrepareCluster prepares a k8s cluster for e2e tests based on the provided configuration
// This function orchestrates the installation of Istio, deployment of telemetry manager,
// and setup of test prerequisites in the same order as the GitHub workflow
func PrepareCluster(t TestingT, k8sClient client.Client, cfg Config) error {
	ctx := t.Context()

	t.Logf("Preparing cluster: image=%s, istio=%t, fips=%t, experimental=%t",
		cfg.ManagerImage, cfg.InstallIstio, cfg.OperateInFIPSMode, cfg.EnableExperimental)

	// 1. Validate configuration
	if err := validateConfig(cfg); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// 2. Ensure kyma-system namespace exists
	if err := ensureNamespace(ctx, k8sClient, kymaSystemNamespace, nil); err != nil {
		return fmt.Errorf("failed to ensure kyma-system namespace: %w", err)
	}

	// 3. Install Istio if requested (BEFORE manager)
	if cfg.InstallIstio {
		installIstio(t, k8sClient)
	}

	// 4. Deploy telemetry manager (includes CRDs and PriorityClass from helm chart)
	if !cfg.SkipManagerDeployment {
		if err := deployManager(t, k8sClient, cfg); err != nil {
			return fmt.Errorf("failed to deploy telemetry manager: %w", err)
		}
	}

	// 5. Deploy test prerequisites (AFTER manager - needs CRDs)
	if !cfg.SkipPrerequisites && !cfg.SkipManagerDeployment {
		if err := deployTestPrerequisites(t, k8sClient); err != nil {
			return fmt.Errorf("failed to deploy test prerequisites: %w", err)
		}
	}

	t.Log("Cluster preparation complete")
	return nil
}

// validateConfig validates the cluster preparation configuration
func validateConfig(cfg Config) error {
	if cfg.ManagerImage == "" && !cfg.SkipManagerDeployment {
		return fmt.Errorf("MANAGER_IMAGE is required when manager deployment is not skipped")
	}
	return nil
}

// ensureNamespace creates a namespace if it doesn't exist (internal helper using context directly)
func ensureNamespace(ctx context.Context, k8sClient client.Client, name string, labels map[string]string) error {
	return ensureNamespaceInternal(ctx, k8sClient, name, labels)
}
