package kubeprep

import (
	"context"
	"fmt"
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
func installIstio(t TestingT, k8sClient client.Client) error {
	t.Helper()
	ctx := t.Context()

	t.Log("Installing Istio...")

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

	t.Logf("Using Istio version: %s", version)

	// 1. Apply istio-manager.yaml
	managerURL := fmt.Sprintf(istioManagerReleaseURLTmpl, version)
	t.Logf("Applying Istio manager from %s", managerURL)
	if err := applyFromURL(ctx, k8sClient, t, managerURL); err != nil {
		return fmt.Errorf("failed to apply Istio manager: %w", err)
	}

	// 2. Apply istio-default-cr.yaml
	crURL := fmt.Sprintf(istioCRReleaseURLTmpl, version)
	t.Logf("Applying Istio default CR from %s", crURL)
	if err := applyFromURL(ctx, k8sClient, t, crURL); err != nil {
		return fmt.Errorf("failed to apply Istio default CR: %w", err)
	}

	// 3. Wait for istiod deployment
	t.Log("Waiting for istiod deployment to be ready...")
	if err := waitForIstiod(ctx, k8sClient); err != nil {
		return fmt.Errorf("istiod not ready: %w", err)
	}

	// 4. Verify Istio is fully operational
	t.Log("Verifying Istio is fully operational...")
	if err := verifyIstioOperational(ctx, k8sClient, t); err != nil {
		return fmt.Errorf("Istio not fully operational: %w", err)
	}

	// 5. Apply Istio Telemetry CR with retry
	t.Log("Applying Istio Telemetry CR...")
	if err := applyIstioTelemetry(ctx, k8sClient, t); err != nil {
		return fmt.Errorf("failed to apply Istio Telemetry: %w", err)
	}

	// 6. Apply PeerAuthentication for istio-system (STRICT mTLS)
	t.Log("Applying PeerAuthentication for istio-system (STRICT mTLS)...")
	if err := applyPeerAuthentication(ctx, k8sClient, t, "default", istioNamespace, "STRICT"); err != nil {
		return fmt.Errorf("failed to apply PeerAuthentication for istio-system: %w", err)
	}

	// 7. Create istio-permissive-mtls namespace with istio-injection label
	t.Log("Creating istio-permissive-mtls namespace...")
	if err := createNamespaceWithIstioInjection(ctx, k8sClient, istioPermissiveNamespace); err != nil {
		return fmt.Errorf("failed to create istio-permissive-mtls namespace: %w", err)
	}

	// 8. Wait for default service account
	t.Log("Waiting for default service account in istio-permissive-mtls...")
	if err := waitForServiceAccount(ctx, k8sClient, "default", istioPermissiveNamespace, 60*time.Second); err != nil {
		return fmt.Errorf("default service account not ready: %w", err)
	}

	// 9. Apply PeerAuthentication for istio-permissive-mtls (PERMISSIVE mTLS)
	t.Log("Applying PeerAuthentication for istio-permissive-mtls (PERMISSIVE mTLS)...")
	if err := applyPeerAuthentication(ctx, k8sClient, t, "default", istioPermissiveNamespace, "PERMISSIVE"); err != nil {
		return fmt.Errorf("failed to apply PeerAuthentication for istio-permissive-mtls: %w", err)
	}

	t.Log("Istio installation complete and verified")
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
func applyIstioTelemetry(ctx context.Context, k8sClient client.Client, t TestingT) error {
	t.Helper()

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
		return applyYAML(ctx, k8sClient, t, telemetryYAML)
	})
}

// applyPeerAuthentication applies a PeerAuthentication resource with retry logic
func applyPeerAuthentication(ctx context.Context, k8sClient client.Client, t TestingT, name, namespace, mode string) error {
	t.Helper()

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
		return applyYAML(ctx, k8sClient, t, peerAuthYAML)
	})
}

// createNamespaceWithIstioInjection creates a namespace with Istio injection enabled
func createNamespaceWithIstioInjection(ctx context.Context, k8sClient client.Client, name string) error {
	return ensureNamespaceInternal(ctx, k8sClient, name, map[string]string{
		"istio-injection": "enabled",
	})
}

// verifyIstioOperational checks that Istio is ready to handle traffic
func verifyIstioOperational(ctx context.Context, k8sClient client.Client, t TestingT) error {
	t.Helper()

	// Check that istiod is ready
	if err := waitForDeployment(ctx, k8sClient, "istiod", istioNamespace, 2*time.Minute); err != nil {
		return err
	}

	// Check that webhook configurations are in place
	// This ensures Istio can inject sidecars
	t.Log("Checking Istio mutating webhook...")
	webhook := &admissionv1.MutatingWebhookConfiguration{}
	webhookKey := types.NamespacedName{Name: "istio-sidecar-injector"}

	return retryWithBackoff(ctx, 10, 5*time.Second, func() error {
		if err := k8sClient.Get(ctx, webhookKey, webhook); err != nil {
			return fmt.Errorf("webhook not ready: %w", err)
		}
		t.Log("Istio mutating webhook is ready")
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
func uninstallIstio(t TestingT, k8sClient client.Client) error {
	t.Helper()
	ctx := t.Context()

	t.Log("Uninstalling Istio...")

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

	t.Logf("Using Istio version for cleanup: %s", version)

	// Step 1: Delete all resources of CRDs with group containing "istio.io"
	t.Log("Step 1: Deleting all Istio CR instances (*.istio.io)...")
	if err := deleteIstioResources(ctx, k8sClient, t); err != nil {
		t.Logf("Warning: failed to delete Istio resources: %v", err)
	}

	// Wait for all Istio resources to be fully deleted
	t.Log("Waiting for all Istio resources to be deleted...")
	if err := waitForIstioResourcesDeletion(ctx, k8sClient, t); err != nil {
		t.Logf("Warning: timeout waiting for Istio resources deletion: %v", err)
		// Continue anyway - best effort
	}

	// Step 2: Delete the Istio CR (istios.operator.kyma-project.io)
	// This must happen AFTER all istio.io resources are gone
	t.Log("Step 2: Deleting Istio CR (istios.operator.kyma-project.io)...")
	crURL := fmt.Sprintf(istioCRReleaseURLTmpl, version)
	t.Logf("Deleting Istio CR from %s", crURL)
	if err := deleteFromURL(ctx, k8sClient, crURL); err != nil {
		t.Logf("Warning: failed to delete Istio CR: %v", err)
	}

	// Wait for Istio CR to be fully deleted (operator cleans up components)
	t.Log("Waiting for Istio CR to be deleted and operator to clean up components...")
	if err := waitForIstioCRDeletion(ctx, k8sClient, t); err != nil {
		t.Logf("Warning: timeout waiting for Istio CR deletion: %v", err)
		// Continue anyway - best effort
	}

	// Step 3: Delete remaining resources (operator, manager, namespaces)
	// This must happen AFTER Istio CRs are gone to avoid finalizer issues
	t.Log("Step 3: Deleting Istio manager and namespaces...")

	// Delete Istio manager (operator) - safe now, Istio CRs are gone
	managerURL := fmt.Sprintf(istioManagerReleaseURLTmpl, version)
	t.Logf("Deleting Istio manager from %s", managerURL)
	if err := deleteFromURL(ctx, k8sClient, managerURL); err != nil {
		t.Logf("Warning: failed to delete Istio manager: %v", err)
	}

	// Delete istio-permissive-mtls namespace
	t.Log("Deleting istio-permissive-mtls namespace...")
	if err := deleteNamespace(ctx, k8sClient, istioPermissiveNamespace); err != nil {
		t.Logf("Warning: failed to delete istio-permissive-mtls namespace: %v", err)
	}

	// Delete istio-system namespace (should be mostly empty by now)
	t.Log("Deleting istio-system namespace...")
	if err := deleteNamespace(ctx, k8sClient, istioNamespace); err != nil {
		t.Logf("Warning: failed to delete istio-system namespace: %v", err)
	}

	t.Log("Istio uninstallation complete")
	return nil
}

// waitForIstioResourcesDeletion waits for all Istio resources (*.istio.io) to be deleted
func waitForIstioResourcesDeletion(ctx context.Context, k8sClient client.Client, t TestingT) error {
	t.Helper()

	maxAttempts := 30
	delaySeconds := 2 * time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Get current Istio CRDs
		istioCRDs, err := getIstioCRDs(ctx, k8sClient, t)
		if err != nil {
			return fmt.Errorf("failed to get Istio CRDs: %w", err)
		}

		if len(istioCRDs) == 0 {
			t.Log("All Istio CRDs are gone")
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
				t.Logf("Warning: failed to count %s.%s: %v", crd.Plural, crd.Group, err)
				continue
			}
			totalResources += count
		}

		if totalResources == 0 {
			t.Log("All Istio resource instances are deleted")
			return nil
		}

		t.Logf("Waiting for %d Istio resources to be deleted (attempt %d/%d)...", totalResources, attempt, maxAttempts)
		time.Sleep(delaySeconds)
	}

	return fmt.Errorf("timeout: Istio resources still exist after %d attempts", maxAttempts)
}

// waitForIstioCRDeletion waits for Istio CRs (istios.operator.kyma-project.io) to be deleted
func waitForIstioCRDeletion(ctx context.Context, k8sClient client.Client, t TestingT) error {
	t.Helper()

	maxAttempts := 60 // 2 minutes
	delaySeconds := 2 * time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Check if any Istio CRs exist
		// Kind for istios is "Istio" (singular, capitalized)
		count, err := countResourcesByGVRK(ctx, k8sClient, "operator.kyma-project.io", "v1alpha2", "istios", "Istio")
		if err != nil {
			// CRD might not exist anymore, which is fine
			if isNotFoundError(err) {
				t.Log("Istio CRD no longer exists (already deleted)")
				return nil
			}
			t.Logf("Warning: failed to check Istio CRs: %v", err)
		}

		if count == 0 {
			t.Log("All Istio CRs are deleted, operator has cleaned up components")
			return nil
		}

		t.Logf("Waiting for %d Istio CR(s) to be deleted (attempt %d/%d)...", count, attempt, maxAttempts)
		time.Sleep(delaySeconds)
	}

	return fmt.Errorf("timeout: Istio CRs still exist after %d attempts", maxAttempts)
}

// deleteIstioResources deletes all resources from CRDs with group containing "istio.io"
// This dynamically queries the API server for all CRDs and finds matching ones
func deleteIstioResources(ctx context.Context, k8sClient client.Client, t TestingT) error {
	t.Helper()

	t.Log("Querying API server for Istio CRDs...")

	// Get all CRDs with "istio.io" in the group
	istioCRDs, err := getIstioCRDs(ctx, k8sClient, t)
	if err != nil {
		return fmt.Errorf("failed to get Istio CRDs: %w", err)
	}

	if len(istioCRDs) == 0 {
		t.Log("No Istio CRDs found, skipping resource cleanup")
		return nil
	}

	t.Logf("Found %d Istio CRDs, deleting their resources...", len(istioCRDs))

	// Delete all resources for each CRD
	for _, crd := range istioCRDs {
		// Use the first stored version (preferred version)
		version := ""
		if len(crd.Versions) > 0 {
			version = crd.Versions[0]
		}
		if version == "" {
			t.Logf("Warning: CRD %s has no versions, skipping", crd.Name)
			continue
		}

		t.Logf("Deleting %s.%s resources...", crd.Plural, crd.Group)
		if err := deleteAllResourcesByGVRK(ctx, k8sClient, t, crd.Group, version, crd.Plural, crd.Kind); err != nil {
			t.Logf("Warning: failed to delete %s.%s: %v", crd.Plural, crd.Group, err)
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
func getIstioCRDs(ctx context.Context, k8sClient client.Client, t TestingT) ([]IstioCRD, error) {
	t.Helper()

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

		t.Logf("Found Istio CRD: %s (%s.%s, kind=%s)", name, plural, group, kind)
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
