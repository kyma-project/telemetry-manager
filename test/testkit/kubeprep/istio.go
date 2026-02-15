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
	ctx := t.Context()

	t.Log("Installing Istio...")

	// Ensure kyma-system namespace exists (Istio manager uses it)
	if err := ensureNamespaceInternal(ctx, k8sClient, kymaSystemNamespace, nil); err != nil {
		return fmt.Errorf("failed to ensure kyma-system namespace: %w", err)
	}

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

	// 1. Apply istio-manager.yaml
	managerURL := fmt.Sprintf(istioManagerReleaseURLTmpl, version)
	if err := applyFromURL(ctx, k8sClient, t, managerURL); err != nil {
		return fmt.Errorf("failed to apply Istio manager: %w", err)
	}

	// 2. Apply istio-default-cr.yaml
	crURL := fmt.Sprintf(istioCRReleaseURLTmpl, version)
	if err := applyFromURL(ctx, k8sClient, t, crURL); err != nil {
		return fmt.Errorf("failed to apply Istio default CR: %w", err)
	}

	// 3. Wait for istiod deployment
	if err := waitForIstiod(ctx, k8sClient); err != nil {
		return fmt.Errorf("istiod not ready: %w", err)
	}

	// 4. Verify Istio is fully operational
	if err := verifyIstioOperational(ctx, k8sClient); err != nil {
		return fmt.Errorf("Istio not fully operational: %w", err)
	}

	// 5. Apply Istio Telemetry CR with retry
	if err := applyIstioTelemetry(ctx, k8sClient, t); err != nil {
		return fmt.Errorf("failed to apply Istio Telemetry: %w", err)
	}

	// 6. Apply PeerAuthentication for istio-system (STRICT mTLS)
	if err := applyPeerAuthentication(ctx, k8sClient, t, "default", istioNamespace, "STRICT"); err != nil {
		return fmt.Errorf("failed to apply PeerAuthentication for istio-system: %w", err)
	}

	// 7. Create istio-permissive-mtls namespace with istio-injection label
	if err := createNamespaceWithIstioInjection(ctx, k8sClient, istioPermissiveNamespace); err != nil {
		return fmt.Errorf("failed to create istio-permissive-mtls namespace: %w", err)
	}

	// 8. Wait for default service account
	if err := waitForServiceAccount(ctx, k8sClient, "default", istioPermissiveNamespace, 60*time.Second); err != nil {
		return fmt.Errorf("default service account not ready: %w", err)
	}

	// 9. Apply PeerAuthentication for istio-permissive-mtls (PERMISSIVE mTLS)
	if err := applyPeerAuthentication(ctx, k8sClient, t, "default", istioPermissiveNamespace, "PERMISSIVE"); err != nil {
		return fmt.Errorf("failed to apply PeerAuthentication for istio-permissive-mtls: %w", err)
	}

	t.Log("Istio installed")
	return nil
}

// waitForIstiod waits for the istiod deployment to be ready
func waitForIstiod(ctx context.Context, k8sClient client.Client) error {
	return retryWithBackoff(ctx, 10, 30*time.Second, func() error {
		return waitForDeployment(ctx, k8sClient, "istiod", istioNamespace, 30*time.Second)
	})
}

// applyIstioTelemetry applies the Istio Telemetry CR with retry logic
func applyIstioTelemetry(ctx context.Context, k8sClient client.Client, t TestingT) error {
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
func verifyIstioOperational(ctx context.Context, k8sClient client.Client) error {
	// Check that istiod is ready
	if err := waitForDeployment(ctx, k8sClient, "istiod", istioNamespace, 2*time.Minute); err != nil {
		return err
	}

	// Check that webhook configurations are in place
	webhook := &admissionv1.MutatingWebhookConfiguration{}
	webhookKey := types.NamespacedName{Name: "istio-sidecar-injector"}

	return retryWithBackoff(ctx, 10, 5*time.Second, func() error {
		return k8sClient.Get(ctx, webhookKey, webhook)
	})
}

// uninstallIstio removes Istio from the cluster following the proper cleanup order
func uninstallIstio(t TestingT, k8sClient client.Client) error {
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

	// Step 1: Delete all resources of CRDs with group containing "istio.io"
	_ = deleteIstioResources(ctx, k8sClient)

	// Wait for all Istio resources to be fully deleted
	_ = waitForIstioResourcesDeletion(ctx, k8sClient)

	// Step 2: Delete the Istio CR (istios.operator.kyma-project.io)
	crURL := fmt.Sprintf(istioCRReleaseURLTmpl, version)
	_ = deleteFromURL(ctx, k8sClient, crURL)

	// Wait for Istio CR to be fully deleted
	_ = waitForIstioCRDeletion(ctx, k8sClient)

	// Step 3: Delete remaining resources (operator, manager, namespaces)
	managerURL := fmt.Sprintf(istioManagerReleaseURLTmpl, version)
	_ = deleteFromURL(ctx, k8sClient, managerURL)

	_ = deleteNamespace(ctx, k8sClient, istioPermissiveNamespace)
	_ = deleteNamespace(ctx, k8sClient, istioNamespace)

	return nil
}

// waitForIstioResourcesDeletion waits for all Istio resources (*.istio.io) to be deleted
func waitForIstioResourcesDeletion(ctx context.Context, k8sClient client.Client) error {
	maxAttempts := 30
	delaySeconds := 2 * time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		istioCRDs, err := getIstioCRDs(ctx, k8sClient)
		if err != nil {
			return fmt.Errorf("failed to get Istio CRDs: %w", err)
		}

		if len(istioCRDs) == 0 {
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
				continue
			}
			totalResources += count
		}

		if totalResources == 0 {
			return nil
		}

		time.Sleep(delaySeconds)
	}

	return fmt.Errorf("timeout: Istio resources still exist after %d attempts", maxAttempts)
}

// waitForIstioCRDeletion waits for Istio CRs (istios.operator.kyma-project.io) to be deleted
func waitForIstioCRDeletion(ctx context.Context, k8sClient client.Client) error {
	maxAttempts := 60
	delaySeconds := 2 * time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		count, err := countResourcesByGVRK(ctx, k8sClient, "operator.kyma-project.io", "v1alpha2", "istios", "Istio")
		if err != nil {
			if isNotFoundError(err) {
				return nil
			}
		}

		if count == 0 {
			return nil
		}

		time.Sleep(delaySeconds)
	}

	return fmt.Errorf("timeout: Istio CRs still exist after %d attempts", maxAttempts)
}

// deleteIstioResources deletes all resources from CRDs with group containing "istio.io"
func deleteIstioResources(ctx context.Context, k8sClient client.Client) error {
	istioCRDs, err := getIstioCRDs(ctx, k8sClient)
	if err != nil {
		return fmt.Errorf("failed to get Istio CRDs: %w", err)
	}

	if len(istioCRDs) == 0 {
		return nil
	}

	for _, crd := range istioCRDs {
		version := ""
		if len(crd.Versions) > 0 {
			version = crd.Versions[0]
		}
		if version == "" {
			continue
		}

		_ = deleteAllResourcesByGVRK(ctx, k8sClient, crd.Group, version, crd.Plural, crd.Kind)
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

		if !strings.Contains(group, "istio.io") {
			continue
		}

		name, _, _ := unstructured.NestedString(item.Object, "metadata", "name")
		plural, _, _ := unstructured.NestedString(item.Object, "spec", "names", "plural")
		kind, _, _ := unstructured.NestedString(item.Object, "spec", "names", "kind")

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
