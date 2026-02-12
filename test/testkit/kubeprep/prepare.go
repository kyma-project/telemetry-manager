package kubeprep

import (
	"context"
	"fmt"
	"log"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PrepareCluster prepares a k8s cluster for e2e tests based on the provided configuration
// This function orchestrates the installation of Istio, deployment of telemetry manager,
// and setup of test prerequisites in the same order as the GitHub workflow
func PrepareCluster(ctx context.Context, k8sClient client.Client, cfg Config) error {
	log.Printf("Preparing cluster with configuration: %+v", cfg)

	// 1. Validate configuration
	if err := validateConfig(cfg); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// 2. Ensure kyma-system namespace exists
	log.Println("Ensuring kyma-system namespace exists...")
	if err := ensureNamespace(ctx, k8sClient, kymaSystemNamespace, nil); err != nil {
		return fmt.Errorf("failed to ensure kyma-system namespace: %w", err)
	}

	// 3. Install Istio if requested (BEFORE manager)
	if cfg.InstallIstio {
		if err := installIstio(ctx, k8sClient); err != nil {
			return fmt.Errorf("failed to install Istio: %w", err)
		}
	} else {
		log.Println("Skipping Istio installation (INSTALL_ISTIO=false)")
	}

	// 4. Deploy telemetry manager (includes CRDs and PriorityClass from helm chart)
	if !cfg.SkipManagerDeployment {
		if err := deployManager(ctx, k8sClient, cfg); err != nil {
			return fmt.Errorf("failed to deploy telemetry manager: %w", err)
		}
	} else {
		log.Println("Skipping telemetry manager deployment (SKIP_MANAGER_DEPLOYMENT=true)")
	}

	// 5. Deploy test prerequisites (AFTER manager - needs CRDs)
	if !cfg.SkipPrerequisites && !cfg.SkipManagerDeployment {
		if err := deployTestPrerequisites(ctx, k8sClient); err != nil {
			return fmt.Errorf("failed to deploy test prerequisites: %w", err)
		}
	} else if cfg.SkipPrerequisites {
		log.Println("Skipping test prerequisites deployment (SKIP_PREREQUISITES=true)")
	}

	log.Println("Cluster preparation complete")
	return nil
}

// validateConfig validates the cluster preparation configuration
func validateConfig(cfg Config) error {
	if cfg.ManagerImage == "" && !cfg.SkipManagerDeployment {
		return fmt.Errorf("MANAGER_IMAGE is required when manager deployment is not skipped")
	}
	return nil
}
