//go:build e2e

package e2e

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/gomega"
)

const (
	serviceAccountName        = "testsuite"
	serviceAccountBindingName = "sa-testsuite"
)

func fetchAuthToken(ctx context.Context, k8sClient client.Client) string {
	var (
		sa     corev1.ServiceAccount
		secret corev1.Secret
	)

	Eventually(func(g Gomega) bool {
		g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: serviceAccountName, Namespace: defaultNamespaceName}, &sa)).To(Succeed())
		g.Expect(sa.Secrets).To(HaveLen(1))
		return true
	}, timeout, interval).Should(BeTrue())

	Eventually(func(g Gomega) bool {
		g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: sa.Secrets[0].Name, Namespace: defaultNamespaceName}, &secret)).To(Succeed())
		g.Expect(secret.Data["token"]).NotTo(BeEmpty())
		return true
	}, timeout, interval).Should(BeTrue())

	return string(secret.Data["token"])
}

func deployAuthToken(ctx context.Context, k8sClient client.Client) {
	sa := corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName, Namespace: defaultNamespaceName}}
	_ = k8sClient.Create(ctx, &sa) // Deliberately ignore an error, because the ServiceAccount can exist already.

	crb := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: serviceAccountBindingName, Namespace: defaultNamespaceName},
		Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: sa.ObjectMeta.Name, Namespace: sa.ObjectMeta.Namespace}},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "cluster-admin"},
	}
	_ = k8sClient.Create(ctx, &crb) // Deliberately ignore an error, because the ClusterRoleBinding can exist already.
}
