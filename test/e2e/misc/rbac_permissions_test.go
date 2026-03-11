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

func TestRBACPermissions(t *testing.T) {
	suite.SetupTest(t, suite.LabelTelemetry, suite.LabelMisc)

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
	createServiceAccount(testNS, "pipeline-creator-sa")

	// Create role bindings
	setupViewerRoleBindings(testNS)
	setupEditorRoleBindings(testNS)
	setupAdminRoleBindings(testNS)
	setupTelemetryOnlyEditorRoleBindings(testNS)
	setupPipelineCreatorRoleBindings(testNS)

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

		// Test custom pipeline creator permissions
		testPipelineCreatorPermissions(g, testNS)
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

func setupPipelineCreatorRoleBindings(testNS string) {
	// Create custom role for pipeline creation only
	creatorRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("test-pipeline-creator-%s", testNS),
		},
		Rules: []rbacv1.PolicyRule{{
			APIGroups: []string{"telemetry.kyma-project.io"},
			Resources: []string{"logpipelines", "metricpipelines", "tracepipelines"},
			Verbs:     []string{"create", "get", "list"},
		}},
	}
	Expect(suite.K8sClient.Create(suite.Ctx, creatorRole)).To(Succeed())

	creatorBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("test-rbac-creator-%s", testNS),
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "pipeline-creator-sa",
			Namespace: testNS,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     creatorRole.Name,
		},
	}
	Expect(suite.K8sClient.Create(suite.Ctx, creatorBinding)).To(Succeed())
}

func testViewerPermissions(g Gomega, testNS string) {
	user := fmt.Sprintf("system:serviceaccount:%s:viewer-sa", testNS)

	// Should be able to view pipelines
	checkCanI(g, user, "get", "logpipelines", "", true, "Viewer can get LogPipelines")
	checkCanI(g, user, "list", "logpipelines", "", true, "Viewer can list LogPipelines")
	checkCanI(g, user, "watch", "logpipelines", "", true, "Viewer can watch LogPipelines")

	checkCanI(g, user, "get", "metricpipelines", "", true, "Viewer can get MetricPipelines")
	checkCanI(g, user, "list", "metricpipelines", "", true, "Viewer can list MetricPipelines")

	checkCanI(g, user, "get", "tracepipelines", "", true, "Viewer can get TracePipelines")
	checkCanI(g, user, "list", "tracepipelines", "", true, "Viewer can list TracePipelines")

	checkCanI(g, user, "get", "telemetries", "", true, "Viewer can get Telemetries")
	checkCanI(g, user, "list", "telemetries", "", true, "Viewer can list Telemetries")

	// Should be able to view ConfigMaps
	checkCanI(g, user, "get", "configmaps", kitkyma.SystemNamespaceName, true, "Viewer can get ConfigMaps")
	checkCanI(g, user, "list", "configmaps", kitkyma.SystemNamespaceName, true, "Viewer can list ConfigMaps")

	// Should NOT be able to view Secrets
	checkCanI(g, user, "get", "secrets", kitkyma.SystemNamespaceName, false, "Viewer cannot get Secrets")
	checkCanI(g, user, "list", "secrets", kitkyma.SystemNamespaceName, false, "Viewer cannot list Secrets")
	checkCanI(g, user, "create", "secrets", kitkyma.SystemNamespaceName, false, "Viewer cannot create Secrets")

	// Should NOT be able to create/update/delete pipelines
	checkCanI(g, user, "create", "logpipelines", "", false, "Viewer cannot create LogPipelines")
	checkCanI(g, user, "update", "logpipelines", "", false, "Viewer cannot update LogPipelines")
	checkCanI(g, user, "delete", "logpipelines", "", false, "Viewer cannot delete LogPipelines")

	// Status subresources - viewer can read status
	checkCanI(g, user, "get", "logpipelines/status", "", true, "Viewer can get LogPipeline status")
	checkCanI(g, user, "get", "metricpipelines/status", "", true, "Viewer can get MetricPipeline status")
	checkCanI(g, user, "get", "tracepipelines/status", "", true, "Viewer can get TracePipeline status")
	checkCanI(g, user, "get", "telemetries/status", "", true, "Viewer can get Telemetry status")

	// But cannot update status (controller-managed)
	checkCanI(g, user, "update", "logpipelines/status", "", false, "Viewer cannot update LogPipeline status")
	checkCanI(g, user, "patch", "logpipelines/status", "", false, "Viewer cannot patch LogPipeline status")

	// Finalizers - viewer cannot modify
	checkCanI(g, user, "update", "logpipelines/finalizers", "", false, "Viewer cannot update LogPipeline finalizers")
}

func testEditorPermissions(g Gomega, testNS string) {
	user := fmt.Sprintf("system:serviceaccount:%s:editor-sa", testNS)

	// Should be able to view pipelines
	checkCanI(g, user, "get", "logpipelines", "", true, "Editor can get LogPipelines")
	checkCanI(g, user, "list", "logpipelines", "", true, "Editor can list LogPipelines")

	// Should be able to create/update/delete pipelines
	checkCanI(g, user, "create", "logpipelines", "", true, "Editor can create LogPipelines")
	checkCanI(g, user, "update", "logpipelines", "", true, "Editor can update LogPipelines")
	checkCanI(g, user, "patch", "logpipelines", "", true, "Editor can patch LogPipelines")
	checkCanI(g, user, "delete", "logpipelines", "", true, "Editor can delete LogPipelines")

	checkCanI(g, user, "create", "metricpipelines", "", true, "Editor can create MetricPipelines")
	checkCanI(g, user, "delete", "metricpipelines", "", true, "Editor can delete MetricPipelines")

	checkCanI(g, user, "create", "tracepipelines", "", true, "Editor can create TracePipelines")
	checkCanI(g, user, "delete", "tracepipelines", "", true, "Editor can delete TracePipelines")

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

	// Status subresources - editor can read but NOT update (controller-managed)
	checkCanI(g, user, "get", "logpipelines/status", "", true, "Editor can get LogPipeline status")
	checkCanI(g, user, "update", "logpipelines/status", "", false, "Editor cannot update LogPipeline status (controller-managed)")
	checkCanI(g, user, "patch", "logpipelines/status", "", false, "Editor cannot patch LogPipeline status (controller-managed)")

	checkCanI(g, user, "get", "telemetries/status", "", true, "Editor can get Telemetry status")
	checkCanI(g, user, "update", "telemetries/status", "", false, "Editor cannot update Telemetry status (controller-managed)")

	// Finalizers - editor CAN update finalizers (needed for deletion with finalizers)
	checkCanI(g, user, "update", "logpipelines/finalizers", "", true, "Editor can update LogPipeline finalizers")
	checkCanI(g, user, "update", "metricpipelines/finalizers", "", true, "Editor can update MetricPipeline finalizers")
	checkCanI(g, user, "update", "tracepipelines/finalizers", "", true, "Editor can update TracePipeline finalizers")
}

func testAdminPermissions(g Gomega, testNS string) {
	user := fmt.Sprintf("system:serviceaccount:%s:admin-sa", testNS)

	// Should have all editor permissions for pipelines
	checkCanI(g, user, "get", "logpipelines", "", true, "Admin can get LogPipelines")
	checkCanI(g, user, "create", "logpipelines", "", true, "Admin can create LogPipelines")
	checkCanI(g, user, "delete", "logpipelines", "", true, "Admin can delete LogPipelines")

	checkCanI(g, user, "create", "metricpipelines", "", true, "Admin can create MetricPipelines")
	checkCanI(g, user, "delete", "tracepipelines", "", true, "Admin can delete TracePipelines")

	// Telemetry CR - same restrictions as editor (Lifecycle Manager-owned)
	checkCanI(g, user, "patch", "telemetries", "", true, "Admin can patch Telemetries")
	checkCanI(g, user, "create", "telemetries", "", false, "Admin cannot create Telemetries (Lifecycle Manager-owned)")
	checkCanI(g, user, "delete", "telemetries", "", false, "Admin cannot delete Telemetries (Lifecycle Manager-owned)")

	// Admin DOES have Secret access (from base K8s admin role)
	checkCanI(g, user, "get", "secrets", kitkyma.SystemNamespaceName, true, "Admin can get Secrets (from base admin role)")
	checkCanI(g, user, "create", "secrets", kitkyma.SystemNamespaceName, true, "Admin can create Secrets (from base admin role)")

	// Status subresources - admin can read but NOT update (controller-managed)
	checkCanI(g, user, "get", "logpipelines/status", "", true, "Admin can get LogPipeline status")
	checkCanI(g, user, "update", "logpipelines/status", "", false, "Admin cannot update LogPipeline status (controller-managed)")

	// Finalizers - admin CAN update finalizers
	checkCanI(g, user, "update", "logpipelines/finalizers", "", true, "Admin can update LogPipeline finalizers")
}

func testTelemetryOnlyEditorPermissions(g Gomega, testNS string) {
	user := fmt.Sprintf("system:serviceaccount:%s:telemetry-only-editor-sa", testNS)

	// Should be able to manage telemetry pipelines (from kyma-telemetry-edit)
	checkCanI(g, user, "get", "logpipelines", "", true, "Telemetry-only editor can get LogPipelines")
	checkCanI(g, user, "create", "logpipelines", "", true, "Telemetry-only editor can create LogPipelines")
	checkCanI(g, user, "update", "logpipelines", "", true, "Telemetry-only editor can update LogPipelines")
	checkCanI(g, user, "delete", "logpipelines", "", true, "Telemetry-only editor can delete LogPipelines")

	checkCanI(g, user, "create", "metricpipelines", "", true, "Telemetry-only editor can create MetricPipelines")
	checkCanI(g, user, "delete", "tracepipelines", "", true, "Telemetry-only editor can delete TracePipelines")

	// Telemetry CR - can patch/update but NOT create/delete (Lifecycle Manager-owned)
	checkCanI(g, user, "get", "telemetries", "", true, "Telemetry-only editor can get Telemetries")
	checkCanI(g, user, "patch", "telemetries", "", true, "Telemetry-only editor can patch Telemetries")
	checkCanI(g, user, "create", "telemetries", "", false, "Telemetry-only editor CANNOT create Telemetries")
	checkCanI(g, user, "delete", "telemetries", "", false, "Telemetry-only editor CANNOT delete Telemetries")

	// CRITICAL TEST: Should NOT have Secret access (kyma-telemetry-edit doesn't grant it)
	checkCanI(g, user, "get", "secrets", kitkyma.SystemNamespaceName, false, "Telemetry-only editor CANNOT get Secrets")
	checkCanI(g, user, "list", "secrets", kitkyma.SystemNamespaceName, false, "Telemetry-only editor CANNOT list Secrets")
	checkCanI(g, user, "create", "secrets", kitkyma.SystemNamespaceName, false, "Telemetry-only editor CANNOT create Secrets")
	checkCanI(g, user, "update", "secrets", kitkyma.SystemNamespaceName, false, "Telemetry-only editor CANNOT update Secrets")
	checkCanI(g, user, "delete", "secrets", kitkyma.SystemNamespaceName, false, "Telemetry-only editor CANNOT delete Secrets")

	// Status subresources - can read but NOT update (controller-managed)
	checkCanI(g, user, "get", "logpipelines/status", "", true, "Telemetry-only editor can get LogPipeline status")
	checkCanI(g, user, "update", "logpipelines/status", "", false, "Telemetry-only editor CANNOT update LogPipeline status")
	checkCanI(g, user, "patch", "telemetries/status", "", false, "Telemetry-only editor CANNOT patch Telemetry status")

	// Finalizers - CAN update finalizers (needed for deletion)
	checkCanI(g, user, "update", "logpipelines/finalizers", "", true, "Telemetry-only editor can update LogPipeline finalizers")
	checkCanI(g, user, "update", "metricpipelines/finalizers", "", true, "Telemetry-only editor can update MetricPipeline finalizers")
}

func testPipelineCreatorPermissions(g Gomega, testNS string) {
	user := fmt.Sprintf("system:serviceaccount:%s:pipeline-creator-sa", testNS)

	// Should be able to create and view pipelines
	checkCanI(g, user, "create", "logpipelines", "", true, "Pipeline creator can create LogPipelines")
	checkCanI(g, user, "get", "logpipelines", "", true, "Pipeline creator can get LogPipelines")
	checkCanI(g, user, "list", "logpipelines", "", true, "Pipeline creator can list LogPipelines")

	// Should NOT be able to update/delete pipelines
	checkCanI(g, user, "update", "logpipelines", "", false, "Pipeline creator cannot update LogPipelines")
	checkCanI(g, user, "delete", "logpipelines", "", false, "Pipeline creator cannot delete LogPipelines")

	// Status subresources - can read status
	checkCanI(g, user, "get", "logpipelines/status", "", true, "Pipeline creator can get LogPipeline status")

	// Finalizers - cannot update finalizers (no delete permission)
	checkCanI(g, user, "update", "logpipelines/finalizers", "", false, "Pipeline creator cannot update LogPipeline finalizers")
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

	// Only fail if we couldn't parse yes/no from the output
	if result == "" {
		g.Expect(err).NotTo(HaveOccurred(), "kubectl auth can-i failed to return yes/no for: %s. Output: %s", description, output)
		return
	}

	if expected {
		g.Expect(result).To(Equal("yes"), description)
	} else {
		g.Expect(result).To(Equal("no"), description)
	}
}

func cleanupRoleBindings(t *testing.T, testNS string) {
	// Cleanup ClusterRoleBindings
	bindings := []string{
		fmt.Sprintf("test-rbac-viewer-%s", testNS),
		fmt.Sprintf("test-rbac-editor-%s", testNS),
		fmt.Sprintf("test-rbac-admin-%s", testNS),
		fmt.Sprintf("test-rbac-telemetry-only-%s", testNS),
		fmt.Sprintf("test-rbac-creator-%s", testNS),
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

	// Cleanup custom ClusterRole
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("test-pipeline-creator-%s", testNS),
		},
	}

	err := suite.K8sClient.Delete(suite.Ctx, cr)
	if err != nil && !apierrors.IsNotFound(err) {
		t.Logf("Failed to delete ClusterRole: %v", err)
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
