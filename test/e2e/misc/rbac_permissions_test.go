package misc

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var resourceTypes = []string{"logpipelines", "metricpipelines", "tracepipelines", "telemetries"}

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
	Expect(kitk8s.CreateObjects(t,
		kitk8sobjects.NewServiceAccount("viewer-sa", testNS).K8sObject(),
		kitk8sobjects.NewServiceAccount("editor-sa", testNS).K8sObject(),
		kitk8sobjects.NewServiceAccount("admin-sa", testNS).K8sObject(),
		kitk8sobjects.NewServiceAccount("telemetry-only-editor-sa", testNS).K8sObject(),
	)).To(Succeed())

	// Create all role bindings
	setupAllRoleBindings(testNS)

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

func clusterRoleBindingName(persona, testNS string) string {
	return fmt.Sprintf("test-rbac-%s-%s", persona, testNS)
}

func roleBindingName(persona, testNS string) string {
	return fmt.Sprintf("test-rbac-%s-ns-%s", persona, testNS)
}

func setupAllRoleBindings(testNS string) {
	// Viewer bindings
	viewClusterBinding := kitk8sobjects.NewClusterRoleBinding(
		clusterRoleBindingName("viewer", testNS),
		kitk8sobjects.WithClusterRoleRef("view"),
		kitk8sobjects.WithServiceAccountSubject("viewer-sa", testNS),
	).K8sObject()
	viewNSBinding := kitk8sobjects.NewRoleBinding(
		roleBindingName("viewer", testNS),
		kitkyma.SystemNamespaceName,
		kitk8sobjects.WithClusterRoleAsRoleRef("view"),
		kitk8sobjects.WithServiceAccountSubjectForRole("viewer-sa", testNS),
	).K8sObject()

	// Editor bindings
	editClusterBinding := kitk8sobjects.NewClusterRoleBinding(
		clusterRoleBindingName("editor", testNS),
		kitk8sobjects.WithClusterRoleRef("edit"),
		kitk8sobjects.WithServiceAccountSubject("editor-sa", testNS),
	).K8sObject()
	editNSBinding := kitk8sobjects.NewRoleBinding(
		roleBindingName("editor", testNS),
		kitkyma.SystemNamespaceName,
		kitk8sobjects.WithClusterRoleAsRoleRef("edit"),
		kitk8sobjects.WithServiceAccountSubjectForRole("editor-sa", testNS),
	).K8sObject()

	// Admin bindings
	adminClusterBinding := kitk8sobjects.NewClusterRoleBinding(
		clusterRoleBindingName("admin", testNS),
		kitk8sobjects.WithClusterRoleRef("admin"),
		kitk8sobjects.WithServiceAccountSubject("admin-sa", testNS),
	).K8sObject()

	// Telemetry-only editor bindings (ONLY kyma-telemetry-edit, without base edit role)
	telemetryOnlyBinding := kitk8sobjects.NewClusterRoleBinding(
		clusterRoleBindingName("telemetry-only", testNS),
		kitk8sobjects.WithClusterRoleRef("kyma-telemetry-edit"),
		kitk8sobjects.WithServiceAccountSubject("telemetry-only-editor-sa", testNS),
	).K8sObject()

	Expect(suite.K8sClient.Create(suite.Ctx, viewClusterBinding)).To(Succeed())
	Expect(suite.K8sClient.Create(suite.Ctx, viewNSBinding)).To(Succeed())
	Expect(suite.K8sClient.Create(suite.Ctx, editClusterBinding)).To(Succeed())
	Expect(suite.K8sClient.Create(suite.Ctx, editNSBinding)).To(Succeed())
	Expect(suite.K8sClient.Create(suite.Ctx, adminClusterBinding)).To(Succeed())
	Expect(suite.K8sClient.Create(suite.Ctx, telemetryOnlyBinding)).To(Succeed())
}

func testViewerPermissions(g Gomega, testNS string) {
	user := fmt.Sprintf("system:serviceaccount:%s:viewer-sa", testNS)

	for _, resourceType := range resourceTypes {
		// Should be able to view telemetry resources
		checkReadPermissions(g, user, resourceType, "", "Viewer")

		// Should NOT be able to create/update/delete telemetry resources
		checkNoWritePermissions(g, user, resourceType, "", "Viewer")

		// Finalizers - viewer cannot modify (finalizers part of metadata, needs update permission)
		checkFinalizerPermissions(g, user, resourceType, "Viewer", false)
	}

	// Should be able to view ConfigMaps
	checkCanI(g, user, "get", "configmaps/telemetry-metricpipelines", kitkyma.SystemNamespaceName, "Viewer", true)
	checkCanI(g, user, "list", "configmaps/telemetry-metricpipelines", kitkyma.SystemNamespaceName, "Viewer", true)

	checkCanI(g, user, "get", "configmaps/telemetry-logpipelines", kitkyma.SystemNamespaceName, "Viewer", true)
	checkCanI(g, user, "list", "configmaps/telemetry-logpipelines", kitkyma.SystemNamespaceName, "Viewer", true)

	checkCanI(g, user, "get", "configmaps/telemetry-tracepipelines", kitkyma.SystemNamespaceName, "Viewer", true)
	checkCanI(g, user, "list", "configmaps/telemetry-tracepipelines", kitkyma.SystemNamespaceName, "Viewer", true)

	// Should NOT be able to view Secrets
	checkNoSecretAccess(g, user, "Viewer")
}

func testEditorPermissions(g Gomega, testNS string) {
	user := fmt.Sprintf("system:serviceaccount:%s:editor-sa", testNS)

	for _, resourceType := range resourceTypes {
		// Should have full CRUD permissions
		checkFullCRUDPermissions(g, user, resourceType, "", "Editor")

		// Finalizers - editor CAN update finalizers (finalizers part of metadata, granted by update permission on resource)
		checkFinalizerPermissions(g, user, resourceType, "Editor", true)
	}
}

func testAdminPermissions(g Gomega, testNS string) {
	user := fmt.Sprintf("system:serviceaccount:%s:admin-sa", testNS)

	for _, resourceType := range resourceTypes {
		// Should have all the  permissions for all telemetry resources
		checkFullCRUDPermissions(g, user, resourceType, "", "Admin")

		// Finalizers - admin CAN update finalizers
		checkFinalizerPermissions(g, user, resourceType, "Admin", true)
	}
}

func testTelemetryOnlyEditorPermissions(g Gomega, testNS string) {
	user := fmt.Sprintf("system:serviceaccount:%s:telemetry-only-editor-sa", testNS)

	for _, resourceTyoe := range resourceTypes {
		// Should be able to manage telemetry resources (from kyma-telemetry-edit)
		checkCanI(g, user, "create", resourceTyoe, "", "Telemetry-only editor", true)
		checkCanI(g, user, "update", resourceTyoe, "", "Telemetry-only editor", true)
		checkCanI(g, user, "patch", resourceTyoe, "", "Telemetry-only editor", true)
		checkCanI(g, user, "delete", resourceTyoe, "", "Telemetry-only editor", true)
		checkCanI(g, user, "deletecollection", resourceTyoe, "", "Telemetry-only editor", true)

		// Finalizers - CAN update finalizers (finalizers part of metadata, granted by update permission)
		checkFinalizerPermissions(g, user, resourceTyoe, "Telemetry-only editor", true)
		finalizerResource := resourceTyoe + "/finalizers"
		checkCanI(g, user, "delete", finalizerResource, "", "Telemetry-only editor", true)
	}

	// Should NOT have Secret access (kyma-telemetry-edit doesn't grant it)
	checkNoSecretAccess(g, user, "Telemetry-only editor")
}

func checkCanI(g Gomega, user, verb, resource, namespace, persona string, expected bool) {
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

	// Build description based on expected result
	action := verb
	if expected {
		description := fmt.Sprintf("%s can %s %s", persona, action, resource)

		// Only fail if we couldn't parse yes/no from the output
		if result == "" {
			g.Expect(err).NotTo(HaveOccurred(),
				"kubectl auth can-i failed to return yes/no\nTest: %s\nCommand: %s\nOutput: %s",
				description, commandStr, output)

			return
		}

		g.Expect(result).To(Equal("yes"),
			"Permission check failed\nTest: %s\nCommand: %s\nExpected: yes\nActual: %s",
			description, commandStr, result)
	} else {
		description := fmt.Sprintf("%s cannot %s %s", persona, action, resource)

		// Only fail if we couldn't parse yes/no from the output
		if result == "" {
			g.Expect(err).NotTo(HaveOccurred(),
				"kubectl auth can-i failed to return yes/no\nTest: %s\nCommand: %s\nOutput: %s",
				description, commandStr, output)

			return
		}

		g.Expect(result).To(Equal("no"),
			"Permission check failed\nTest: %s\nCommand: %s\nExpected: no\nActual: %s",
			description, commandStr, result)
	}
}

func cleanupRoleBindings(t *testing.T, testNS string) {
	// Cleanup ClusterRoleBindings
	bindings := []string{
		clusterRoleBindingName("viewer", testNS),
		clusterRoleBindingName("editor", testNS),
		clusterRoleBindingName("admin", testNS),
		clusterRoleBindingName("telemetry-only", testNS),
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
		roleBindingName("viewer", testNS),
		roleBindingName("editor", testNS),
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

// Common permission check patterns
func checkReadPermissions(g Gomega, user, resource, namespace, persona string) {
	checkCanI(g, user, "get", resource, namespace, persona, true)
	checkCanI(g, user, "list", resource, namespace, persona, true)
	checkCanI(g, user, "watch", resource, namespace, persona, true)
}

func checkNoWritePermissions(g Gomega, user, resource, namespace, persona string) {
	checkCanI(g, user, "create", resource, namespace, persona, false)
	checkCanI(g, user, "update", resource, namespace, persona, false)
	checkCanI(g, user, "delete", resource, namespace, persona, false)
}

func checkFullCRUDPermissions(g Gomega, user, resource, namespace, persona string) {
	checkCanI(g, user, "get", resource, namespace, persona, true)
	checkCanI(g, user, "list", resource, namespace, persona, true)
	checkCanI(g, user, "create", resource, namespace, persona, true)
	checkCanI(g, user, "update", resource, namespace, persona, true)
	checkCanI(g, user, "patch", resource, namespace, persona, true)
	checkCanI(g, user, "delete", resource, namespace, persona, true)
	checkCanI(g, user, "deletecollection", resource, namespace, persona, true)
}

func checkFinalizerPermissions(g Gomega, user, pipelineType, persona string, canUpdate bool) {
	finalizerResource := pipelineType + "/finalizers"
	checkCanI(g, user, "update", finalizerResource, "", persona, canUpdate)
}

func checkNoSecretAccess(g Gomega, user, persona string) {
	checkCanI(g, user, "get", "secrets", kitkyma.SystemNamespaceName, persona, false)
	checkCanI(g, user, "list", "secrets", kitkyma.SystemNamespaceName, persona, false)
	checkCanI(g, user, "create", "secrets", kitkyma.SystemNamespaceName, persona, false)
	checkCanI(g, user, "update", "secrets", kitkyma.SystemNamespaceName, persona, false)
	checkCanI(g, user, "delete", "secrets", kitkyma.SystemNamespaceName, persona, false)
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
