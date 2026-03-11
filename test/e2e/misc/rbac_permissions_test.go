package misc

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var pipelineTypes = []string{"logpipelines", "metricpipelines", "tracepipelines"}

func TestRBACPermissions(t *testing.T) {
	//suite.SetupTest(t, suite.LabelTelemetry, suite.LabelMisc)
	RegisterTestingT(t)
	testNS := suite.IDWithSuffix("rbac-perm")

	// Create test namespace
	namespace := kitk8sobjects.NewNamespace(testNS).K8sObject()
	Expect(kitk8s.CreateObjects(t, namespace)).To(Succeed())

	defer func() {
		// Cleanup bindings
		cleanupRoleBindings(t, testNS)
		Expect(suite.K8sClient.Delete(suite.Ctx, namespace)).To(Succeed())
	}()

	// Create test service accounts
	createServiceAccount(testNS, "viewer-sa")
	createServiceAccount(testNS, "editor-sa")
	createServiceAccount(testNS, "admin-sa")
	createServiceAccount(testNS, "telemetry-only-editor-sa")

	// Create role bindings
	setupViewerRoleBindings(testNS)
	setupEditorRoleBindings(testNS)
	setupAdminRoleBindings(testNS)
	setupTelemetryOnlyEditorRoleBindings(testNS)

	// Wait for RBAC to propagate
	Eventually(func(g Gomega) {
		// Test viewer permissions
		testViewerPermissions(g, testNS)

		// Test editor permissions
		testEditorPermissions(g, testNS)

		// Test admin permissions
		testAdminPermissions(g, testNS)

		// Test telemetry-only editor permissions (verifies kyma-telemetry-edit doesn't grant Secret access)
		testTelemetryOnlyEditorPermissions(g, testNS)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func createServiceAccount(namespace, name string) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	Expect(suite.K8sClient.Create(suite.Ctx, sa)).To(Succeed())
}

func setupViewerRoleBindings(testNS string) {
	// Bind view ClusterRole (which aggregates kyma-telemetry-view)
	viewClusterBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("test-rbac-viewer-%s", testNS),
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "viewer-sa",
			Namespace: testNS,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "view",
		},
	}
	Expect(suite.K8sClient.Create(suite.Ctx, viewClusterBinding)).To(Succeed())

	// Bind view role for namespace-scoped resources
	viewNSBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("test-rbac-viewer-ns-%s", testNS),
			Namespace: kitkyma.SystemNamespaceName,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "viewer-sa",
			Namespace: testNS,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "view",
		},
	}
	Expect(suite.K8sClient.Create(suite.Ctx, viewNSBinding)).To(Succeed())
}

func setupEditorRoleBindings(testNS string) {
	// Bind edit ClusterRole (which aggregates kyma-telemetry-edit)
	editClusterBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("test-rbac-editor-%s", testNS),
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "editor-sa",
			Namespace: testNS,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "edit",
		},
	}
	Expect(suite.K8sClient.Create(suite.Ctx, editClusterBinding)).To(Succeed())

	// Bind edit role for namespace-scoped resources
	editNSBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("test-rbac-editor-ns-%s", testNS),
			Namespace: kitkyma.SystemNamespaceName,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "editor-sa",
			Namespace: testNS,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "edit",
		},
	}
	Expect(suite.K8sClient.Create(suite.Ctx, editNSBinding)).To(Succeed())
}

func setupAdminRoleBindings(testNS string) {
	adminClusterBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("test-rbac-admin-%s", testNS),
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "admin-sa",
			Namespace: testNS,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "admin",
		},
	}
	Expect(suite.K8sClient.Create(suite.Ctx, adminClusterBinding)).To(Succeed())
}

func setupTelemetryOnlyEditorRoleBindings(testNS string) {
	// Bind ONLY kyma-telemetry-edit (without base edit role)
	// This test verifies kyma-telemetry-edit doesn't grant Secret access
	telemetryOnlyBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("test-rbac-telemetry-only-%s", testNS),
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "telemetry-only-editor-sa",
			Namespace: testNS,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "kyma-telemetry-edit",
		},
	}
	Expect(suite.K8sClient.Create(suite.Ctx, telemetryOnlyBinding)).To(Succeed())
}

func testViewerPermissions(g Gomega, testNS string) {
	user := fmt.Sprintf("system:serviceaccount:%s:viewer-sa", testNS)

	for _, pipelineType := range pipelineTypes {
		// Should be able to view pipelines
		checkCanI(g, user, "get", pipelineType, "", true, fmt.Sprintf("Viewer can get %s", pipelineType))
		checkCanI(g, user, "list", pipelineType, "", true, fmt.Sprintf("Viewer can list %s", pipelineType))
		checkCanI(g, user, "watch", pipelineType, "", true, fmt.Sprintf("Viewer can watch %s", pipelineType))

		// Should NOT be able to create/update/delete pipelines
		checkCanI(g, user, "create", pipelineType, "", false, fmt.Sprintf("Viewer cannot create %s", pipelineType))
		checkCanI(g, user, "update", pipelineType, "", false, fmt.Sprintf("Viewer cannot update %s", pipelineType))
		checkCanI(g, user, "delete", pipelineType, "", false, fmt.Sprintf("Viewer cannot delete %s", pipelineType))

		// Finalizers - viewer cannot modify (finalizers part of metadata, needs update permission)
		finalizerResource := pipelineType + "/finalizers"
		checkCanI(g, user, "update", finalizerResource, "", false, fmt.Sprintf("Viewer cannot update %s finalizers", pipelineType))
	}

	// Telemetry CR permissions
	checkCanI(g, user, "get", "telemetries", "", true, "Viewer can get Telemetries")
	checkCanI(g, user, "list", "telemetries", "", true, "Viewer can list Telemetries")

	// Should be able to view ConfigMaps
	checkCanI(g, user, "get", "configmaps/telemetry-metricpipelines", kitkyma.SystemNamespaceName, true, "Viewer can get ConfigMaps")
	checkCanI(g, user, "list", "configmaps/telemetry-metricpipelines", kitkyma.SystemNamespaceName, true, "Viewer can list ConfigMaps")
	checkCanI(g, user, "get", "configmaps/telemetry-logpipelines", kitkyma.SystemNamespaceName, true, "Viewer can get ConfigMaps")
	checkCanI(g, user, "list", "configmaps/telemetry-logpipelines", kitkyma.SystemNamespaceName, true, "Viewer can list ConfigMaps")
	checkCanI(g, user, "get", "configmaps/telemetry-tracepipelines", kitkyma.SystemNamespaceName, true, "Viewer can get ConfigMaps")
	checkCanI(g, user, "list", "configmaps/telemetry-tracepipelines", kitkyma.SystemNamespaceName, true, "Viewer can list ConfigMaps")

	// Should NOT be able to view Secrets
	checkCanI(g, user, "get", "secrets", kitkyma.SystemNamespaceName, false, "Viewer cannot get Secrets")
	checkCanI(g, user, "list", "secrets", kitkyma.SystemNamespaceName, false, "Viewer cannot list Secrets")
	checkCanI(g, user, "create", "secrets", kitkyma.SystemNamespaceName, false, "Viewer cannot create Secrets")
}

func testEditorPermissions(g Gomega, testNS string) {
	user := fmt.Sprintf("system:serviceaccount:%s:editor-sa", testNS)

	for _, pipelineType := range pipelineTypes {
		// Should be able to view pipelines
		checkCanI(g, user, "get", pipelineType, "", true, fmt.Sprintf("Editor can get %s", pipelineType))
		checkCanI(g, user, "list", pipelineType, "", true, fmt.Sprintf("Editor can list %s", pipelineType))

		// Should be able to create/update/delete pipelines
		checkCanI(g, user, "create", pipelineType, "", true, fmt.Sprintf("Editor can create %s", pipelineType))
		checkCanI(g, user, "update", pipelineType, "", true, fmt.Sprintf("Editor can update %s", pipelineType))
		checkCanI(g, user, "patch", pipelineType, "", true, fmt.Sprintf("Editor can patch %s", pipelineType))
		checkCanI(g, user, "delete", pipelineType, "", true, fmt.Sprintf("Editor can delete %s", pipelineType))

		// Finalizers - editor CAN update finalizers (finalizers part of metadata, granted by update permission on resource)
		finalizerResource := pipelineType + "/finalizers"
		checkCanI(g, user, "update", finalizerResource, "", true, fmt.Sprintf("Editor can update %s finalizers", pipelineType))
	}

	// Telemetry CR (operator.kyma-project.io) - can update/patch but NOT create/delete
	// (Lifecycle Manager owns create/delete operations)
	checkCanI(g, user, "get", "telemetries", "", true, "Editor can get Telemetries")
	checkCanI(g, user, "patch", "telemetries", "", true, "Editor can patch Telemetries")
	checkCanI(g, user, "update", "telemetries", "", true, "Editor can update Telemetries")
	checkCanI(g, user, "create", "telemetries", "", false, "Editor cannot create Telemetries (Lifecycle Manager-owned)")
	checkCanI(g, user, "delete", "telemetries", "", false, "Editor cannot delete Telemetries (Lifecycle Manager-owned)")

	// Editor CAN manage Secrets (from base K8s edit role, NOT from kyma-telemetry-edit)
	checkCanI(g, user, "get", "secrets", kitkyma.SystemNamespaceName, true, "Editor can get Secrets (from base edit role)")
	checkCanI(g, user, "create", "secrets", kitkyma.SystemNamespaceName, true, "Editor can create Secrets (from base edit role)")
	checkCanI(g, user, "delete", "secrets", kitkyma.SystemNamespaceName, true, "Editor can delete Secrets (from base edit role)")

	// Should not be able to delete or update ConfigMaps in telemetry namespace.
	checkCanI(g, user, "delete", "configmaps", kitkyma.SystemNamespaceName, true, "Editor can delete ConfigMaps (from base edit role)")
	checkCanI(g, user, "update", "configmaps", kitkyma.SystemNamespaceName, true, "Editor can update ConfigMaps (from base edit role)")
}

func testAdminPermissions(g Gomega, testNS string) {
	user := fmt.Sprintf("system:serviceaccount:%s:admin-sa", testNS)

	for _, pipelineType := range pipelineTypes {
		// Should have all editor permissions for pipelines
		checkCanI(g, user, "get", pipelineType, "", true, fmt.Sprintf("Admin can get %s", pipelineType))
		checkCanI(g, user, "create", pipelineType, "", true, fmt.Sprintf("Admin can create %s", pipelineType))
		checkCanI(g, user, "delete", pipelineType, "", true, fmt.Sprintf("Admin can delete %s", pipelineType))

		// Finalizers - admin CAN update finalizers
		finalizerResource := pipelineType + "/finalizers"
		checkCanI(g, user, "update", finalizerResource, "", true, fmt.Sprintf("Admin can update %s finalizers", pipelineType))
	}

	// Telemetry CR - same restrictions as editor (Lifecycle Manager-owned)
	checkCanI(g, user, "patch", "telemetries", "", true, "Admin can patch Telemetries")
	checkCanI(g, user, "create", "telemetries", "", false, "Admin cannot create Telemetries (Lifecycle Manager-owned)")
	checkCanI(g, user, "delete", "telemetries", "", false, "Admin cannot delete Telemetries (Lifecycle Manager-owned)")

}

func testTelemetryOnlyEditorPermissions(g Gomega, testNS string) {
	user := fmt.Sprintf("system:serviceaccount:%s:telemetry-only-editor-sa", testNS)

	for _, pipelineType := range pipelineTypes {
		// Should be able to manage telemetry pipelines (from kyma-telemetry-edit)
		checkCanI(g, user, "get", pipelineType, "", true, fmt.Sprintf("Telemetry-only editor can get %s", pipelineType))
		checkCanI(g, user, "create", pipelineType, "", true, fmt.Sprintf("Telemetry-only editor can create %s", pipelineType))
		checkCanI(g, user, "update", pipelineType, "", true, fmt.Sprintf("Telemetry-only editor can update %s", pipelineType))
		checkCanI(g, user, "delete", pipelineType, "", true, fmt.Sprintf("Telemetry-only editor can delete %s", pipelineType))

		// Finalizers - CAN update finalizers (finalizers part of metadata, granted by update permission)
		finalizerResource := pipelineType + "/finalizers"
		checkCanI(g, user, "update", finalizerResource, "", true, fmt.Sprintf("Telemetry-only editor can update %s finalizers", pipelineType))
		checkCanI(g, user, "delete", finalizerResource, "", true, fmt.Sprintf("Telemetry-only editor can delete %s finalizers", pipelineType))
	}

	// Telemetry CR - can patch/update but NOT create/delete (Lifecycle Manager-owned)
	checkCanI(g, user, "get", "telemetries", "", true, "Telemetry-only editor can get Telemetries")
	checkCanI(g, user, "patch", "telemetries", "", true, "Telemetry-only editor can patch Telemetries")
	checkCanI(g, user, "create", "telemetries", "", false, "Telemetry-only editor CANNOT create Telemetries")
	checkCanI(g, user, "delete", "telemetries", "", false, "Telemetry-only editor CANNOT delete Telemetries")

	// Should NOT have Secret access (kyma-telemetry-edit doesn't grant it)
	checkCanI(g, user, "get", "secrets", kitkyma.SystemNamespaceName, false, "Telemetry-only editor CANNOT get Secrets")
	checkCanI(g, user, "list", "secrets", kitkyma.SystemNamespaceName, false, "Telemetry-only editor CANNOT list Secrets")
	checkCanI(g, user, "create", "secrets", kitkyma.SystemNamespaceName, false, "Telemetry-only editor CANNOT create Secrets")
	checkCanI(g, user, "update", "secrets", kitkyma.SystemNamespaceName, false, "Telemetry-only editor CANNOT update Secrets")
	checkCanI(g, user, "delete", "secrets", kitkyma.SystemNamespaceName, false, "Telemetry-only editor CANNOT delete Secrets")
}

func checkCanI(g Gomega, user, verb, resource, namespace string, expected bool, description string) {
	args := []string{"auth", "can-i", verb, resource, "--as", user}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}

	cmd := exec.CommandContext(context.TODO(), "kubectl", args...)
	out, err := cmd.CombinedOutput()

	// kubectl auth can-i returns exit code 0 for "yes" and exit code 1 for "no"
	// Both are valid responses, so we parse the output instead of checking the error
	output := string(out)
	result := parseKubectlAuthCanIOutput(output)

	// Build detailed error message
	commandStr := fmt.Sprintf("kubectl %s", strings.Join(args, " "))

	// Only fail if we couldn't parse yes/no from the output
	if result == "" {
		g.Expect(err).NotTo(HaveOccurred(),
			"kubectl auth can-i failed to return yes/no\nTest: %s\nCommand: %s\nOutput: %s",
			description, commandStr, output)
		return
	}

	expectedStr := "no"
	if expected {
		expectedStr = "yes"
	}

	if expected {
		g.Expect(result).To(Equal("yes"),
			"Permission check failed\nTest: %s\nCommand: %s\nExpected: %s\nActual: %s",
			description, commandStr, expectedStr, result)
	} else {
		g.Expect(result).To(Equal("no"),
			"Permission check failed\nTest: %s\nCommand: %s\nExpected: %s\nActual: %s",
			description, commandStr, expectedStr, result)
	}
}

func cleanupRoleBindings(t *testing.T, testNS string) {
	// Cleanup ClusterRoleBindings
	bindings := []string{
		fmt.Sprintf("test-rbac-viewer-%s", testNS),
		fmt.Sprintf("test-rbac-editor-%s", testNS),
		fmt.Sprintf("test-rbac-admin-%s", testNS),
		fmt.Sprintf("test-rbac-telemetry-only-%s", testNS),
	}

	for _, name := range bindings {
		crb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: name},
		}

		err := suite.K8sClient.Delete(suite.Ctx, crb)
		if err != nil && !apierrors.IsNotFound(err) {
			t.Logf("Failed to delete ClusterRoleBinding %s: %v", name, err)
		}
	}

	// Cleanup RoleBindings
	nsBindings := []string{
		fmt.Sprintf("test-rbac-viewer-ns-%s", testNS),
		fmt.Sprintf("test-rbac-editor-ns-%s", testNS),
	}

	for _, name := range nsBindings {
		rb := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: kitkyma.SystemNamespaceName,
			},
		}

		err := suite.K8sClient.Delete(suite.Ctx, rb)
		if err != nil && !apierrors.IsNotFound(err) {
			t.Logf("Failed to delete RoleBinding %s: %v", name, err)
		}
	}
}

// parseKubectlAuthCanIOutput extracts the yes/no result from kubectl auth can-i output,
// filtering out warnings and other non-result lines.
func parseKubectlAuthCanIOutput(output string) string {
	lines := strings.SplitSeq(output, "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if line == "yes" || line == "no" {
			return line
		}
	}

	return ""
}
