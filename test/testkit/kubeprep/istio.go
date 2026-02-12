package kubeprep

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	istioVersion               = "1.25.3" // Default version, can be overridden by .env
	istioManagerReleaseURLTmpl = "https://github.com/kyma-project/istio/releases/download/%s/istio-manager.yaml"
	istioCRReleaseURLTmpl      = "https://github.com/kyma-project/istio/releases/download/%s/istio-default-cr.yaml"
	istioNamespace             = "istio-system"
	istioPermissiveNamespace   = "istio-permissive-mtls"
)

// installIstio installs Istio in the cluster
func installIstio(ctx context.Context, k8sClient client.Client) error {
	log.Println("Installing Istio...")

	// Try to read Istio version from .env file
	version := istioVersion
	envFile := filepath.Join(".", ".env")
	if env, err := loadEnvFile(envFile); err == nil {
		if v, ok := env["ISTIO_VERSION"]; ok && v != "" {
			version = v
		} else if v, ok := env["ENV_ISTIO_VERSION"]; ok && v != "" {
			version = v
		}
	}

	log.Printf("Using Istio version: %s", version)

	// 1. Apply istio-manager.yaml
	managerURL := fmt.Sprintf(istioManagerReleaseURLTmpl, version)
	log.Printf("Applying Istio manager from %s", managerURL)
	if err := applyFromURL(ctx, k8sClient, managerURL); err != nil {
		return fmt.Errorf("failed to apply Istio manager: %w", err)
	}

	// 2. Apply istio-default-cr.yaml
	crURL := fmt.Sprintf(istioCRReleaseURLTmpl, version)
	log.Printf("Applying Istio default CR from %s", crURL)
	if err := applyFromURL(ctx, k8sClient, crURL); err != nil {
		return fmt.Errorf("failed to apply Istio default CR: %w", err)
	}

	// 3. Wait for istiod deployment
	log.Println("Waiting for istiod deployment to be ready...")
	if err := waitForIstiod(ctx, k8sClient); err != nil {
		return fmt.Errorf("istiod not ready: %w", err)
	}

	// 4. Verify Istio is fully operational
	log.Println("Verifying Istio is fully operational...")
	if err := verifyIstioOperational(ctx, k8sClient); err != nil {
		return fmt.Errorf("Istio not fully operational: %w", err)
	}

	// 5. Apply Istio Telemetry CR with retry
	log.Println("Applying Istio Telemetry CR...")
	if err := applyIstioTelemetry(ctx, k8sClient); err != nil {
		return fmt.Errorf("failed to apply Istio Telemetry: %w", err)
	}

	// 6. Apply PeerAuthentication for istio-system (STRICT mTLS)
	log.Println("Applying PeerAuthentication for istio-system (STRICT mTLS)...")
	if err := applyPeerAuthentication(ctx, k8sClient, "default", istioNamespace, "STRICT"); err != nil {
		return fmt.Errorf("failed to apply PeerAuthentication for istio-system: %w", err)
	}

	// 7. Create istio-permissive-mtls namespace with istio-injection label
	log.Println("Creating istio-permissive-mtls namespace...")
	if err := createNamespaceWithIstioInjection(ctx, k8sClient, istioPermissiveNamespace); err != nil {
		return fmt.Errorf("failed to create istio-permissive-mtls namespace: %w", err)
	}

	// 8. Wait for default service account
	log.Println("Waiting for default service account in istio-permissive-mtls...")
	if err := waitForServiceAccount(ctx, k8sClient, "default", istioPermissiveNamespace, 60*time.Second); err != nil {
		return fmt.Errorf("default service account not ready: %w", err)
	}

	// 9. Apply PeerAuthentication for istio-permissive-mtls (PERMISSIVE mTLS)
	log.Println("Applying PeerAuthentication for istio-permissive-mtls (PERMISSIVE mTLS)...")
	if err := applyPeerAuthentication(ctx, k8sClient, "default", istioPermissiveNamespace, "PERMISSIVE"); err != nil {
		return fmt.Errorf("failed to apply PeerAuthentication for istio-permissive-mtls: %w", err)
	}

	log.Println("Istio installation complete and verified")
	return nil
}

// waitForIstiod waits for the istiod deployment to be ready
func waitForIstiod(ctx context.Context, k8sClient client.Client) error {
	return retryWithBackoff(ctx, 10, 30*time.Second, func() error {
		return waitForDeployment(ctx, k8sClient, "istiod", istioNamespace, 30*time.Second)
	})
}

// applyIstioTelemetry applies the Istio Telemetry CR with retry logic
// This matches the configuration in hack/deploy-istio.sh
func applyIstioTelemetry(ctx context.Context, k8sClient client.Client) error {
	telemetryYAML := `apiVersion: telemetry.istio.io/v1
kind: Telemetry
metadata:
  name: access-config
  namespace: istio-system
spec:
  accessLogging:
    - providers:
        - name: stdout-json
        - name: kyma-logs
  tracing:
    - providers:
        - name: kyma-traces
      randomSamplingPercentage: 100.00
`

	return retryWithBackoff(ctx, 10, 30*time.Second, func() error {
		return applyYAML(ctx, k8sClient, telemetryYAML)
	})
}

// applyPeerAuthentication applies a PeerAuthentication resource with retry logic
func applyPeerAuthentication(ctx context.Context, k8sClient client.Client, name, namespace, mode string) error {
	peerAuthYAML := fmt.Sprintf(`apiVersion: security.istio.io/v1
kind: PeerAuthentication
metadata:
  name: %s
  namespace: %s
spec:
  mtls:
    mode: %s
`, name, namespace, mode)

	return retryWithBackoff(ctx, 10, 30*time.Second, func() error {
		return applyYAML(ctx, k8sClient, peerAuthYAML)
	})
}

// createNamespaceWithIstioInjection creates a namespace with Istio injection enabled
func createNamespaceWithIstioInjection(ctx context.Context, k8sClient client.Client, name string) error {
	return ensureNamespace(ctx, k8sClient, name, map[string]string{
		"istio-injection": "enabled",
	})
}

// verifyIstioOperational checks that Istio is ready to handle traffic
func verifyIstioOperational(ctx context.Context, k8sClient client.Client) error {
	// Check that istiod is ready
	if err := waitForDeployment(ctx, k8sClient, "istiod", istioNamespace, 2*time.Minute); err != nil {
		return err
	}

	// Check that webhook configurations are in place
	// This ensures Istio can inject sidecars
	log.Println("Checking Istio mutating webhook...")
	webhook := &admissionv1.MutatingWebhookConfiguration{}
	webhookKey := types.NamespacedName{Name: "istio-sidecar-injector"}

	return retryWithBackoff(ctx, 10, 5*time.Second, func() error {
		if err := k8sClient.Get(ctx, webhookKey, webhook); err != nil {
			return fmt.Errorf("webhook not ready: %w", err)
		}
		log.Println("Istio mutating webhook is ready")
		return nil
	})
}

// uninstallIstio removes Istio from the cluster following the proper cleanup order
// This is a best-effort cleanup that logs warnings but doesn't fail
//
// Cleanup order (critical to avoid hanging resources):
// 1. Delete all resources of CRDs with group containing "istio.io"
//    - Wait until all instances are gone (verify deletion)
// 2. Delete the Istio CRs (istios.operator.kyma-project.io)
//    - Wait until Istio CRs are gone (operator removes components)
// 3. Delete remaining Istio resources once Istio CRs are gone
//    - Delete Istio manager (operator) - safe now, no finalizers blocking
//    - Delete namespaces
func uninstallIstio(ctx context.Context, k8sClient client.Client) error {
	log.Println("Uninstalling Istio...")

	// Try to read Istio version from .env file
	version := istioVersion
	envFile := filepath.Join(".", ".env")
	if env, err := loadEnvFile(envFile); err == nil {
		if v, ok := env["ISTIO_VERSION"]; ok && v != "" {
			version = v
		} else if v, ok := env["ENV_ISTIO_VERSION"]; ok && v != "" {
			version = v
		}
	}

	log.Printf("Using Istio version for cleanup: %s", version)

	// Step 1: Delete all resources of CRDs with group containing "istio.io"
	log.Println("Step 1: Deleting all Istio CR instances (*.istio.io)...")
	if err := deleteIstioResources(ctx, k8sClient); err != nil {
		log.Printf("Warning: failed to delete Istio resources: %v", err)
	}

	// Wait for all Istio resources to be fully deleted
	log.Println("Waiting for all Istio resources to be deleted...")
	if err := waitForIstioResourcesDeletion(ctx, k8sClient); err != nil {
		log.Printf("Warning: timeout waiting for Istio resources deletion: %v", err)
		// Continue anyway - best effort
	}

	// Step 2: Delete the Istio CR (istios.operator.kyma-project.io)
	// This must happen AFTER all istio.io resources are gone
	log.Println("Step 2: Deleting Istio CR (istios.operator.kyma-project.io)...")
	crURL := fmt.Sprintf(istioCRReleaseURLTmpl, version)
	log.Printf("Deleting Istio CR from %s", crURL)
	if err := deleteFromURL(ctx, k8sClient, crURL); err != nil {
		log.Printf("Warning: failed to delete Istio CR: %v", err)
	}

	// Wait for Istio CR to be fully deleted (operator cleans up components)
	log.Println("Waiting for Istio CR to be deleted and operator to clean up components...")
	if err := waitForIstioCRDeletion(ctx, k8sClient); err != nil {
		log.Printf("Warning: timeout waiting for Istio CR deletion: %v", err)
		// Continue anyway - best effort
	}

	// Step 3: Delete remaining resources (operator, manager, namespaces)
	// This must happen AFTER Istio CRs are gone to avoid finalizer issues
	log.Println("Step 3: Deleting Istio manager and namespaces...")

	// Delete Istio manager (operator) - safe now, Istio CRs are gone
	managerURL := fmt.Sprintf(istioManagerReleaseURLTmpl, version)
	log.Printf("Deleting Istio manager from %s", managerURL)
	if err := deleteFromURL(ctx, k8sClient, managerURL); err != nil {
		log.Printf("Warning: failed to delete Istio manager: %v", err)
	}

	// Delete istio-permissive-mtls namespace
	log.Println("Deleting istio-permissive-mtls namespace...")
	if err := deleteNamespace(ctx, k8sClient, istioPermissiveNamespace); err != nil {
		log.Printf("Warning: failed to delete istio-permissive-mtls namespace: %v", err)
	}

	// Delete istio-system namespace (should be mostly empty by now)
	log.Println("Deleting istio-system namespace...")
	if err := deleteNamespace(ctx, k8sClient, istioNamespace); err != nil {
		log.Printf("Warning: failed to delete istio-system namespace: %v", err)
	}

	log.Println("Istio uninstallation complete")
	return nil
}

// waitForIstioResourcesDeletion waits for all Istio resources (*.istio.io) to be deleted
func waitForIstioResourcesDeletion(ctx context.Context, k8sClient client.Client) error {
	maxAttempts := 30
	delaySeconds := 2 * time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Get current Istio CRDs
		istioCRDs, err := getIstioCRDs(ctx, k8sClient)
		if err != nil {
			return fmt.Errorf("failed to get Istio CRDs: %w", err)
		}

		if len(istioCRDs) == 0 {
			log.Println("All Istio CRDs are gone")
			return nil
		}

		// Check if any resources still exist
		totalResources := 0
		for _, crd := range istioCRDs {
			version := ""
			if len(crd.Versions) > 0 {
				version = crd.Versions[0]
			}

			count, err := countResourcesByGVRK(ctx, k8sClient, crd.Group, version, crd.Plural, crd.Kind)
			if err != nil {
				log.Printf("Warning: failed to count %s.%s: %v", crd.Plural, crd.Group, err)
				continue
			}
			totalResources += count
		}

		if totalResources == 0 {
			log.Println("All Istio resource instances are deleted")
			return nil
		}

		log.Printf("Waiting for %d Istio resources to be deleted (attempt %d/%d)...", totalResources, attempt, maxAttempts)
		time.Sleep(delaySeconds)
	}

	return fmt.Errorf("timeout: Istio resources still exist after %d attempts", maxAttempts)
}

// waitForIstioCRDeletion waits for Istio CRs (istios.operator.kyma-project.io) to be deleted
func waitForIstioCRDeletion(ctx context.Context, k8sClient client.Client) error {
	maxAttempts := 60 // 2 minutes
	delaySeconds := 2 * time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Check if any Istio CRs exist
		// Kind for istios is "Istio" (singular, capitalized)
		count, err := countResourcesByGVRK(ctx, k8sClient, "operator.kyma-project.io", "v1alpha2", "istios", "Istio")
		if err != nil {
			// CRD might not exist anymore, which is fine
			if isNotFoundError(err) {
				log.Println("Istio CRD no longer exists (already deleted)")
				return nil
			}
			log.Printf("Warning: failed to check Istio CRs: %v", err)
		}

		if count == 0 {
			log.Println("All Istio CRs are deleted, operator has cleaned up components")
			return nil
		}

		log.Printf("Waiting for %d Istio CR(s) to be deleted (attempt %d/%d)...", count, attempt, maxAttempts)
		time.Sleep(delaySeconds)
	}

	return fmt.Errorf("timeout: Istio CRs still exist after %d attempts", maxAttempts)
}

// deleteIstioResources deletes all resources from CRDs with group containing "istio.io"
// This dynamically queries the API server for all CRDs and finds matching ones
func deleteIstioResources(ctx context.Context, k8sClient client.Client) error {
	log.Println("Querying API server for Istio CRDs...")

	// Get all CRDs with "istio.io" in the group
	istioCRDs, err := getIstioCRDs(ctx, k8sClient)
	if err != nil {
		return fmt.Errorf("failed to get Istio CRDs: %w", err)
	}

	if len(istioCRDs) == 0 {
		log.Println("No Istio CRDs found, skipping resource cleanup")
		return nil
	}

	log.Printf("Found %d Istio CRDs, deleting their resources...", len(istioCRDs))

	// Delete all resources for each CRD
	for _, crd := range istioCRDs {
		// Use the first stored version (preferred version)
		version := ""
		if len(crd.Versions) > 0 {
			version = crd.Versions[0]
		}
		if version == "" {
			log.Printf("Warning: CRD %s has no versions, skipping", crd.Name)
			continue
		}

		log.Printf("Deleting %s.%s resources...", crd.Plural, crd.Group)
		if err := deleteAllResourcesByGVRK(ctx, k8sClient, crd.Group, version, crd.Plural, crd.Kind); err != nil {
			log.Printf("Warning: failed to delete %s.%s: %v", crd.Plural, crd.Group, err)
			// Continue with other resources
		}
	}

	return nil
}

// IstioCRD represents basic info about an Istio CRD
type IstioCRD struct {
	Name     string
	Group    string
	Versions []string
	Plural   string
	Kind     string // Add Kind field
}

// getIstioCRDs queries the API server for all CRDs with "istio.io" in the group
func getIstioCRDs(ctx context.Context, k8sClient client.Client) ([]IstioCRD, error) {
	// List all CRDs
	crdList := &unstructured.UnstructuredList{}
	crdList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apiextensions.k8s.io",
		Version: "v1",
		Kind:    "CustomResourceDefinitionList",
	})

	if err := k8sClient.List(ctx, crdList); err != nil {
		return nil, fmt.Errorf("failed to list CRDs: %w", err)
	}

	var istioCRDs []IstioCRD

	// Filter CRDs with "istio.io" in the group
	for _, item := range crdList.Items {
		group, found, err := unstructured.NestedString(item.Object, "spec", "group")
		if err != nil || !found {
			continue
		}

		// Check if group contains "istio.io"
		if !strings.Contains(group, "istio.io") {
			continue
		}

		// Extract CRD info
		name, _, _ := unstructured.NestedString(item.Object, "metadata", "name")
		plural, _, _ := unstructured.NestedString(item.Object, "spec", "names", "plural")
		kind, _, _ := unstructured.NestedString(item.Object, "spec", "names", "kind")

		// Extract versions
		versionsRaw, found, err := unstructured.NestedSlice(item.Object, "spec", "versions")
		if err != nil || !found {
			continue
		}

		var versions []string
		for _, v := range versionsRaw {
			vMap, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			versionName, ok := vMap["name"].(string)
			if ok {
				versions = append(versions, versionName)
			}
		}

		if len(versions) == 0 {
			continue
		}

		istioCRDs = append(istioCRDs, IstioCRD{
			Name:     name,
			Group:    group,
			Versions: versions,
			Plural:   plural,
			Kind:     kind,
		})

		log.Printf("Found Istio CRD: %s (%s.%s, kind=%s)", name, plural, group, kind)
	}

	return istioCRDs, nil
}

// isIstioInstalled checks if Istio is installed in the cluster
func isIstioInstalled(ctx context.Context, k8sClient client.Client) bool {
	// Check if istio-system namespace exists
	ns := &corev1.Namespace{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: istioNamespace}, ns)
	return err == nil
}
