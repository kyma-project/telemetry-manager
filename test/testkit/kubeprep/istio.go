package kubeprep

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultIstioVersion        = "1.25.3" // Default version, can be overridden by ISTIO_VERSION env var
	istioManagerReleaseURLTmpl = "https://github.com/kyma-project/istio/releases/download/%s/istio-manager.yaml"
	istioCRReleaseURLTmpl      = "https://github.com/kyma-project/istio/releases/download/%s/istio-default-cr.yaml"
	istioNamespace             = "istio-system"
	istioPermissiveNamespace   = "istio-permissive-mtls"
)

// getIstioVersion returns the Istio version from environment or default
func getIstioVersion() string {
	if v := os.Getenv("ISTIO_VERSION"); v != "" {
		return v
	}

	return defaultIstioVersion
}

// installIstio installs Istio in the cluster
func installIstio(t TestingT, k8sClient client.Client) {
	ctx := t.Context()
	g := gomega.NewWithT(t)

	t.Log("Installing Istio...")

	// Ensure kyma-system namespace exists (Istio manager uses it)
	require.NoError(t, ensureNamespaceInternal(ctx, k8sClient, kymaSystemNamespace, nil), "failed to ensure kyma-system namespace")

	version := getIstioVersion()

	// 1. Apply istio-manager.yaml
	managerURL := fmt.Sprintf(istioManagerReleaseURLTmpl, version)
	require.NoError(t, applyFromURL(ctx, k8sClient, managerURL), "failed to apply Istio manager")

	// 2. Apply istio-default-cr.yaml
	crURL := fmt.Sprintf(istioCRReleaseURLTmpl, version)
	require.NoError(t, applyFromURL(ctx, k8sClient, crURL), "failed to apply Istio default CR")

	// 3. Wait for istiod deployment
	t.Log("Waiting for istiod deployment...")
	g.Eventually(func(g gomega.Gomega) {
		g.Expect(waitForDeployment(ctx, k8sClient, "istiod", istioNamespace, 30*time.Second)).To(gomega.Succeed())
	}, 5*time.Minute, 10*time.Second).Should(gomega.Succeed(), "istiod deployment not ready")

	// 4. Wait for webhook configuration
	t.Log("Waiting for Istio webhook...")

	webhook := &admissionregistrationv1.MutatingWebhookConfiguration{}

	g.Eventually(func() error {
		return k8sClient.Get(ctx, types.NamespacedName{Name: "istio-sidecar-injector"}, webhook)
	}, 2*time.Minute, 5*time.Second).Should(gomega.Succeed(), "Istio webhook not ready")

	// 5. Apply Istio Telemetry CR
	t.Log("Applying Istio Telemetry CR...")

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

	g.Eventually(func() error {
		return applyYAML(ctx, k8sClient, telemetryYAML)
	}, 2*time.Minute, 5*time.Second).Should(gomega.Succeed(), "failed to apply Istio Telemetry CR")

	// 6. Apply PeerAuthentication for istio-system (STRICT mTLS)
	t.Log("Applying PeerAuthentication for istio-system...")

	peerAuthIstioYAML := `apiVersion: security.istio.io/v1
kind: PeerAuthentication
metadata:
  name: default
  namespace: istio-system
spec:
  mtls:
    mode: STRICT
`

	g.Eventually(func() error {
		return applyYAML(ctx, k8sClient, peerAuthIstioYAML)
	}, 2*time.Minute, 5*time.Second).Should(gomega.Succeed(), "failed to apply PeerAuthentication for istio-system")

	// 7. Create istio-permissive-mtls namespace with istio-injection label
	require.NoError(t, ensureNamespaceInternal(ctx, k8sClient, istioPermissiveNamespace, map[string]string{
		"istio-injection": "enabled",
	}), "failed to create istio-permissive-mtls namespace")

	// 8. Wait for default service account
	t.Log("Waiting for default service account...")
	g.Eventually(func() error {
		sa := &corev1.ServiceAccount{}
		return k8sClient.Get(ctx, types.NamespacedName{Name: "default", Namespace: istioPermissiveNamespace}, sa)
	}, 60*time.Second, 2*time.Second).Should(gomega.Succeed(), "default service account not ready")

	// 9. Apply PeerAuthentication for istio-permissive-mtls (PERMISSIVE mTLS)
	t.Log("Applying PeerAuthentication for istio-permissive-mtls...")

	peerAuthPermissiveYAML := `apiVersion: security.istio.io/v1
kind: PeerAuthentication
metadata:
  name: default
  namespace: istio-permissive-mtls
spec:
  mtls:
    mode: PERMISSIVE
`

	g.Eventually(func() error {
		return applyYAML(ctx, k8sClient, peerAuthPermissiveYAML)
	}, 2*time.Minute, 5*time.Second).Should(gomega.Succeed(), "failed to apply PeerAuthentication for istio-permissive-mtls")

	t.Log("Istio installed")
}

// uninstallIstio removes Istio from the cluster following the proper cleanup order
func uninstallIstio(t TestingT, k8sClient client.Client) error {
	ctx := t.Context()
	g := gomega.NewWithT(t)

	t.Log("Uninstalling Istio...")

	version := getIstioVersion()

	// Step 1: Delete all resources of CRDs with group containing "istio.io"
	if err := deleteIstioResources(ctx, k8sClient); err != nil {
		return fmt.Errorf("failed to delete Istio resources: %w", err)
	}

	// Wait for all Istio resources to be fully deleted
	t.Log("Waiting for Istio resources deletion...")
	g.Eventually(func() bool {
		istioCRDs, err := getIstioCRDs(ctx, k8sClient)
		if err != nil || len(istioCRDs) == 0 {
			return true
		}

		totalResources := 0

		for _, crd := range istioCRDs {
			if len(crd.Versions) == 0 {
				continue
			}

			count, err := countResourcesByGVRK(ctx, k8sClient, crd.Group, crd.Versions[0], crd.Plural, crd.Kind)
			if err != nil {
				t.Logf("Error counting resources for %s.%s: %v", crd.Plural, crd.Group, err)
				continue
			}

			totalResources += count
		}

		return totalResources == 0
	}, 2*time.Minute, 2*time.Second).Should(gomega.BeTrue(), "Istio resources still exist")

	// Step 2: Delete the Istio CR (istios.operator.kyma-project.io)
	crURL := fmt.Sprintf(istioCRReleaseURLTmpl, version)
	if err := deleteFromURL(ctx, k8sClient, crURL); err != nil {
		return fmt.Errorf("failed to delete Istio CR: %w", err)
	}

	// Wait for Istio CR to be fully deleted
	t.Log("Waiting for Istio CR deletion...")
	g.Eventually(func() bool {
		count, err := countResourcesByGVRK(ctx, k8sClient, "operator.kyma-project.io", "v1alpha2", "istios", "Istio")
		return err != nil || count == 0
	}, 2*time.Minute, 2*time.Second).Should(gomega.BeTrue(), "Istio CR still exists")

	// Step 3: Delete remaining resources (operator, manager, namespaces)
	managerURL := fmt.Sprintf(istioManagerReleaseURLTmpl, version)
	if err := deleteFromURL(ctx, k8sClient, managerURL); err != nil {
		return fmt.Errorf("failed to delete Istio manager: %w", err)
	}

	if err := deleteNamespace(ctx, k8sClient, istioPermissiveNamespace); err != nil {
		return fmt.Errorf("failed to delete namespace %s: %w", istioPermissiveNamespace, err)
	}

	if err := deleteNamespace(ctx, k8sClient, istioNamespace); err != nil {
		return fmt.Errorf("failed to delete namespace %s: %w", istioNamespace, err)
	}

	t.Log("Istio uninstalled")

	return nil
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

		if err := deleteAllResourcesByGVRK(ctx, k8sClient, crd.Group, version, crd.Plural, crd.Kind); err != nil {
			return fmt.Errorf("failed to delete %s.%s resources: %w", crd.Plural, crd.Group, err)
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
	Kind     string
}

// getIstioCRDs queries the API server for all CRDs with "istio.io" in the group
func getIstioCRDs(ctx context.Context, k8sClient client.Client) ([]IstioCRD, error) {
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

	for _, item := range crdList.Items {
		group, found, err := unstructured.NestedString(item.Object, "spec", "group")
		if err != nil || !found {
			continue
		}

		if !strings.Contains(group, "istio.io") {
			continue
		}

		name, _, err := unstructured.NestedString(item.Object, "metadata", "name")
		if err != nil {
			continue
		}

		plural, _, err := unstructured.NestedString(item.Object, "spec", "names", "plural")
		if err != nil {
			continue
		}

		kind, _, err := unstructured.NestedString(item.Object, "spec", "names", "kind")
		if err != nil {
			continue
		}

		versionsRaw, found, err := unstructured.NestedSlice(item.Object, "spec", "versions")
		if err != nil || !found {
			continue
		}

		var versions []string

		for _, v := range versionsRaw {
			vMap, ok := v.(map[string]any)
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
