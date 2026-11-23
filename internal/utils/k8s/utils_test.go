package k8s

import (
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMergePodAnnotations(t *testing.T) {
	tests := []struct {
		name           string
		existing       map[string]string
		desired        map[string]string
		expectedMerged map[string]string
	}{
		{
			name: "should preserve existing kubectl annotations",
			existing: map[string]string{
				"kubectl.kubernetes.io/1": "1",
				"kubectl.kubernetes.io/2": "2",
				"kubectl.kubernetes.io/3": "3",
				"unrelated":               "foo",
			},
			desired: map[string]string{
				"kubectl.kubernetes.io/2": "b",
				"kubectl.kubernetes.io/3": "3",
				"kubectl.kubernetes.io/4": "4",
			},
			expectedMerged: map[string]string{
				"kubectl.kubernetes.io/1": "1",
				"kubectl.kubernetes.io/2": "b",
				"kubectl.kubernetes.io/3": "3",
				"kubectl.kubernetes.io/4": "4",
			},
		},
		{
			name: "should preserve existing checksum annotations",
			existing: map[string]string{
				"checksum/config":   "1",
				"checksum/config-a": "2",
				"checksum/Config":   "3",
				"unrelated":         "foo",
			},
			desired: map[string]string{
				"checksum/config-a": "5",
				"checksum/cOnfig":   "6",
			},
			expectedMerged: map[string]string{
				"checksum/config":   "1",
				"checksum/config-a": "5",
				"checksum/cOnfig":   "6",
			},
		},
		{
			name: "should preserve existing istio restartedAt annotations",
			existing: map[string]string{
				"istio-operator.kyma-project.io/restartedAt": "2023-08-17T13:36:09Z",
				"unrelated": "foo",
			},
			desired: map[string]string{
				"checksum/1": "1",
			},
			expectedMerged: map[string]string{
				"istio-operator.kyma-project.io/restartedAt": "2023-08-17T13:36:09Z",
				"checksum/1": "1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existing := appsv1.Deployment{
				Name:        "some-deployment",
				Annotations: tt.existing,
			}

			desired := appsv1.Deployment{
				Name:        "some-deployment",
				Annotations: tt.desired,
			}

			mergePodAnnotations(&desired.ObjectMeta, existing.ObjectMeta)

			require.Equal(t, len(tt.expectedMerged), len(desired.Annotations))

			for k, v := range tt.expectedMerged {
				require.Contains(t, desired.Annotations, k)
				require.Equal(t, v, desired.Annotations[k])
			}
		})
	}
}

func TestMergeOwnerReference(t *testing.T) {
	oldOwners := []metav1.OwnerReference{
		{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "old-deployment-1",
			UID:        "old-deployment-uid-1",
		},
		{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "old-deployment-2",
			UID:        "old-deployment-uid-2",
		},
	}
	newOwners := []metav1.OwnerReference{
		{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "new-deployment-1",
			UID:        "new-deployment-uid-1",
		},
	}

	merged := mergeOwnerReferences(newOwners, oldOwners)
	require.Equal(t, 3, len(merged))
}

func TestCreateOrUpdateSecret_BackfillsLabelsOnce(t *testing.T) {
	key := types.NamespacedName{Name: "secret", Namespace: "default"}

	existing := &corev1.Secret{
		Name:      key.Name,
		Namespace: key.Namespace,
		Labels:    map[string]string{"existing": "label"},
		Data:      map[string][]byte{"key": []byte("value")},
	}

	fakeClient := fake.NewClientBuilder().WithObjects(existing).Build()

	// reconcile as the telemetry reconciler does: fetch the secret and set the labels explicitly on it
	reconcile := func() {
		var desired corev1.Secret
		require.NoError(t, fakeClient.Get(t.Context(), key, &desired))

		desired.Labels["added"] = "label"

		require.NoError(t, CreateOrUpdateSecret(t.Context(), fakeClient, &desired))
	}

	reconcile()

	var updated corev1.Secret
	require.NoError(t, fakeClient.Get(t.Context(), key, &updated))
	require.Equal(t, map[string]string{"existing": "label", "added": "label"}, updated.Labels)
	require.Equal(t, existing.Data, updated.Data)

	// the second reconcile detects no difference and does not update the secret again
	reconcile()

	var unchanged corev1.Secret
	require.NoError(t, fakeClient.Get(t.Context(), key, &unchanged))
	require.Equal(t, updated.ResourceVersion, unchanged.ResourceVersion)
}

func TestCreateOrUpdateClusterRole_NoOpWhenUnchanged(t *testing.T) {
	key := types.NamespacedName{Name: "cr"}
	existing := &rbacv1.ClusterRole{
		Name: key.Name, Labels: map[string]string{"existing": "label"},
		Rules: []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "list"}}},
	}
	fakeClient := fake.NewClientBuilder().WithObjects(existing).Build()

	var before rbacv1.ClusterRole
	require.NoError(t, fakeClient.Get(t.Context(), key, &before))

	desired := &rbacv1.ClusterRole{
		Name: key.Name, Labels: map[string]string{"existing": "label"},
		Rules: []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "list"}}},
	}
	require.NoError(t, CreateOrUpdateClusterRole(t.Context(), fakeClient, desired))

	var after rbacv1.ClusterRole
	require.NoError(t, fakeClient.Get(t.Context(), key, &after))
	require.Equal(t, before.ResourceVersion, after.ResourceVersion)
}

func TestCreateOrUpdateClusterRole_UpdatesWhenRulesDiffer(t *testing.T) {
	key := types.NamespacedName{Name: "cr"}
	existing := &rbacv1.ClusterRole{
		Name:  key.Name,
		Rules: []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get"}}},
	}
	fakeClient := fake.NewClientBuilder().WithObjects(existing).Build()

	var before rbacv1.ClusterRole
	require.NoError(t, fakeClient.Get(t.Context(), key, &before))

	desired := &rbacv1.ClusterRole{
		Name:  key.Name,
		Rules: []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"pods", "services"}, Verbs: []string{"get", "list"}}},
	}
	require.NoError(t, CreateOrUpdateClusterRole(t.Context(), fakeClient, desired))

	var after rbacv1.ClusterRole
	require.NoError(t, fakeClient.Get(t.Context(), key, &after))
	require.NotEqual(t, before.ResourceVersion, after.ResourceVersion)
	require.Equal(t, []string{"pods", "services"}, after.Rules[0].Resources)
}

func TestCreateOrUpdateRole_NoOpWhenUnchanged(t *testing.T) {
	key := types.NamespacedName{Name: "role", Namespace: "default"}
	existing := &rbacv1.Role{
		Name: key.Name, Namespace: key.Namespace,
		Rules: []rbacv1.PolicyRule{{APIGroups: []string{"coordination.k8s.io"}, Resources: []string{"leases"}, Verbs: []string{"get", "create"}}},
	}
	fakeClient := fake.NewClientBuilder().WithObjects(existing).Build()

	var before rbacv1.Role
	require.NoError(t, fakeClient.Get(t.Context(), key, &before))

	desired := &rbacv1.Role{
		Name: key.Name, Namespace: key.Namespace,
		Rules: []rbacv1.PolicyRule{{APIGroups: []string{"coordination.k8s.io"}, Resources: []string{"leases"}, Verbs: []string{"get", "create"}}},
	}
	require.NoError(t, CreateOrUpdateRole(t.Context(), fakeClient, desired))

	var after rbacv1.Role
	require.NoError(t, fakeClient.Get(t.Context(), key, &after))
	require.Equal(t, before.ResourceVersion, after.ResourceVersion)
}

func TestCreateOrUpdateRole_UpdatesWhenRulesDiffer(t *testing.T) {
	key := types.NamespacedName{Name: "role", Namespace: "default"}
	existing := &rbacv1.Role{
		Name: key.Name, Namespace: key.Namespace,
		Rules: []rbacv1.PolicyRule{{APIGroups: []string{"coordination.k8s.io"}, Resources: []string{"leases"}, Verbs: []string{"get"}}},
	}
	fakeClient := fake.NewClientBuilder().WithObjects(existing).Build()

	var before rbacv1.Role
	require.NoError(t, fakeClient.Get(t.Context(), key, &before))

	desired := &rbacv1.Role{
		Name: key.Name, Namespace: key.Namespace,
		Rules: []rbacv1.PolicyRule{{APIGroups: []string{"coordination.k8s.io"}, Resources: []string{"leases"}, Verbs: []string{"get", "create", "delete"}}},
	}
	require.NoError(t, CreateOrUpdateRole(t.Context(), fakeClient, desired))

	var after rbacv1.Role
	require.NoError(t, fakeClient.Get(t.Context(), key, &after))
	require.NotEqual(t, before.ResourceVersion, after.ResourceVersion)
}

func TestCreateOrUpdateClusterRoleBinding_NoOpWhenUnchanged(t *testing.T) {
	key := types.NamespacedName{Name: "crb"}
	existing := &rbacv1.ClusterRoleBinding{
		Name:     key.Name,
		RoleRef:  rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "foo"},
		Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "sa", Namespace: "default"}},
	}
	fakeClient := fake.NewClientBuilder().WithObjects(existing).Build()

	var before rbacv1.ClusterRoleBinding
	require.NoError(t, fakeClient.Get(t.Context(), key, &before))

	desired := &rbacv1.ClusterRoleBinding{
		Name:     key.Name,
		RoleRef:  rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "foo"},
		Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "sa", Namespace: "default"}},
	}
	require.NoError(t, CreateOrUpdateClusterRoleBinding(t.Context(), fakeClient, desired))

	var after rbacv1.ClusterRoleBinding
	require.NoError(t, fakeClient.Get(t.Context(), key, &after))
	require.Equal(t, before.ResourceVersion, after.ResourceVersion)
}

func TestCreateOrUpdateClusterRoleBinding_UpdatesWhenSubjectsDiffer(t *testing.T) {
	key := types.NamespacedName{Name: "crb"}
	existing := &rbacv1.ClusterRoleBinding{
		Name:     key.Name,
		RoleRef:  rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "foo"},
		Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "sa-old", Namespace: "default"}},
	}
	fakeClient := fake.NewClientBuilder().WithObjects(existing).Build()

	var before rbacv1.ClusterRoleBinding
	require.NoError(t, fakeClient.Get(t.Context(), key, &before))

	desired := &rbacv1.ClusterRoleBinding{
		Name:     key.Name,
		RoleRef:  rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "foo"},
		Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "sa-new", Namespace: "default"}},
	}
	require.NoError(t, CreateOrUpdateClusterRoleBinding(t.Context(), fakeClient, desired))

	var after rbacv1.ClusterRoleBinding
	require.NoError(t, fakeClient.Get(t.Context(), key, &after))
	require.NotEqual(t, before.ResourceVersion, after.ResourceVersion)
	require.Equal(t, "sa-new", after.Subjects[0].Name)
}

func TestCreateOrUpdateRoleBinding_NoOpWhenUnchanged(t *testing.T) {
	key := types.NamespacedName{Name: "rb", Namespace: "default"}
	existing := &rbacv1.RoleBinding{
		Name: key.Name, Namespace: key.Namespace,
		RoleRef:  rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "foo"},
		Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "sa", Namespace: "default"}},
	}
	fakeClient := fake.NewClientBuilder().WithObjects(existing).Build()

	var before rbacv1.RoleBinding
	require.NoError(t, fakeClient.Get(t.Context(), key, &before))

	desired := &rbacv1.RoleBinding{
		Name: key.Name, Namespace: key.Namespace,
		RoleRef:  rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "foo"},
		Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "sa", Namespace: "default"}},
	}
	require.NoError(t, CreateOrUpdateRoleBinding(t.Context(), fakeClient, desired))

	var after rbacv1.RoleBinding
	require.NoError(t, fakeClient.Get(t.Context(), key, &after))
	require.Equal(t, before.ResourceVersion, after.ResourceVersion)
}

func TestCreateOrUpdateRoleBinding_UpdatesWhenSubjectsDiffer(t *testing.T) {
	key := types.NamespacedName{Name: "rb", Namespace: "default"}
	existing := &rbacv1.RoleBinding{
		Name: key.Name, Namespace: key.Namespace,
		RoleRef:  rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "foo"},
		Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "sa-old", Namespace: "default"}},
	}
	fakeClient := fake.NewClientBuilder().WithObjects(existing).Build()

	var before rbacv1.RoleBinding
	require.NoError(t, fakeClient.Get(t.Context(), key, &before))

	desired := &rbacv1.RoleBinding{
		Name: key.Name, Namespace: key.Namespace,
		RoleRef:  rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "foo"},
		Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "sa-new", Namespace: "default"}},
	}
	require.NoError(t, CreateOrUpdateRoleBinding(t.Context(), fakeClient, desired))

	var after rbacv1.RoleBinding
	require.NoError(t, fakeClient.Get(t.Context(), key, &after))
	require.NotEqual(t, before.ResourceVersion, after.ResourceVersion)
	require.Equal(t, "sa-new", after.Subjects[0].Name)
}

func TestCreateOrUpdateServiceAccount_NoOpWhenUnchanged(t *testing.T) {
	key := types.NamespacedName{Name: "sa", Namespace: "default"}
	existing := &corev1.ServiceAccount{
		Name: key.Name, Namespace: key.Namespace, Labels: map[string]string{"app": "foo"},
	}
	fakeClient := fake.NewClientBuilder().WithObjects(existing).Build()

	var before corev1.ServiceAccount
	require.NoError(t, fakeClient.Get(t.Context(), key, &before))

	desired := &corev1.ServiceAccount{
		Name: key.Name, Namespace: key.Namespace, Labels: map[string]string{"app": "foo"},
	}
	require.NoError(t, CreateOrUpdateServiceAccount(t.Context(), fakeClient, desired))

	var after corev1.ServiceAccount
	require.NoError(t, fakeClient.Get(t.Context(), key, &after))
	require.Equal(t, before.ResourceVersion, after.ResourceVersion)
}

func TestCreateOrUpdateServiceAccount_UpdatesWhenLabelsDiffer(t *testing.T) {
	key := types.NamespacedName{Name: "sa", Namespace: "default"}
	existing := &corev1.ServiceAccount{
		Name: key.Name, Namespace: key.Namespace, Labels: map[string]string{"app": "foo"},
	}
	fakeClient := fake.NewClientBuilder().WithObjects(existing).Build()

	var before corev1.ServiceAccount
	require.NoError(t, fakeClient.Get(t.Context(), key, &before))

	desired := &corev1.ServiceAccount{
		Name: key.Name, Namespace: key.Namespace, Labels: map[string]string{"app": "foo", "new": "label"},
	}
	require.NoError(t, CreateOrUpdateServiceAccount(t.Context(), fakeClient, desired))

	var after corev1.ServiceAccount
	require.NoError(t, fakeClient.Get(t.Context(), key, &after))
	require.NotEqual(t, before.ResourceVersion, after.ResourceVersion)
	require.Equal(t, "label", after.Labels["new"])
}

func TestCreateOrUpdateConfigMap_NoOpWhenUnchanged(t *testing.T) {
	key := types.NamespacedName{Name: "cm", Namespace: "default"}
	existing := &corev1.ConfigMap{
		Name: key.Name, Namespace: key.Namespace,
		Data: map[string]string{"key": "value"},
	}
	fakeClient := fake.NewClientBuilder().WithObjects(existing).Build()

	var before corev1.ConfigMap
	require.NoError(t, fakeClient.Get(t.Context(), key, &before))

	desired := &corev1.ConfigMap{
		Name: key.Name, Namespace: key.Namespace,
		Data: map[string]string{"key": "value"},
	}
	require.NoError(t, CreateOrUpdateConfigMap(t.Context(), fakeClient, desired))

	var after corev1.ConfigMap
	require.NoError(t, fakeClient.Get(t.Context(), key, &after))
	require.Equal(t, before.ResourceVersion, after.ResourceVersion)
}

func TestCreateOrUpdateConfigMap_UpdatesWhenDataDiffers(t *testing.T) {
	key := types.NamespacedName{Name: "cm", Namespace: "default"}
	existing := &corev1.ConfigMap{
		Name: key.Name, Namespace: key.Namespace,
		Data: map[string]string{"key": "old"},
	}
	fakeClient := fake.NewClientBuilder().WithObjects(existing).Build()

	var before corev1.ConfigMap
	require.NoError(t, fakeClient.Get(t.Context(), key, &before))

	desired := &corev1.ConfigMap{
		Name: key.Name, Namespace: key.Namespace,
		Data: map[string]string{"key": "new"},
	}
	require.NoError(t, CreateOrUpdateConfigMap(t.Context(), fakeClient, desired))

	var after corev1.ConfigMap
	require.NoError(t, fakeClient.Get(t.Context(), key, &after))
	require.NotEqual(t, before.ResourceVersion, after.ResourceVersion)
	require.Equal(t, "new", after.Data["key"])
}

func TestCreateOrUpdateNetworkPolicy_NoOpWhenUnchanged(t *testing.T) {
	key := types.NamespacedName{Name: "np", Namespace: "default"}
	existing := &networkingv1.NetworkPolicy{
		Name: key.Name, Namespace: key.Namespace,
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "x"}},
		},
	}
	fakeClient := fake.NewClientBuilder().WithObjects(existing).Build()

	var before networkingv1.NetworkPolicy
	require.NoError(t, fakeClient.Get(t.Context(), key, &before))

	desired := &networkingv1.NetworkPolicy{
		Name: key.Name, Namespace: key.Namespace,
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "x"}},
		},
	}
	require.NoError(t, CreateOrUpdateNetworkPolicy(t.Context(), fakeClient, desired))

	var after networkingv1.NetworkPolicy
	require.NoError(t, fakeClient.Get(t.Context(), key, &after))
	require.Equal(t, before.ResourceVersion, after.ResourceVersion)
}

func TestCreateOrUpdateNetworkPolicy_UpdatesWhenSpecDiffers(t *testing.T) {
	key := types.NamespacedName{Name: "np", Namespace: "default"}
	existing := &networkingv1.NetworkPolicy{
		Name: key.Name, Namespace: key.Namespace,
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "x"}},
		},
	}
	fakeClient := fake.NewClientBuilder().WithObjects(existing).Build()

	var before networkingv1.NetworkPolicy
	require.NoError(t, fakeClient.Get(t.Context(), key, &before))

	desired := &networkingv1.NetworkPolicy{
		Name: key.Name, Namespace: key.Namespace,
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "y"}},
		},
	}
	require.NoError(t, CreateOrUpdateNetworkPolicy(t.Context(), fakeClient, desired))

	var after networkingv1.NetworkPolicy
	require.NoError(t, fakeClient.Get(t.Context(), key, &after))
	require.NotEqual(t, before.ResourceVersion, after.ResourceVersion)
}

func TestCreateOrUpdateSecret_NoOpWhenUnchanged(t *testing.T) {
	key := types.NamespacedName{Name: "secret", Namespace: "default"}
	existing := &corev1.Secret{
		Name: key.Name, Namespace: key.Namespace,
		Data: map[string][]byte{"key": []byte("value")},
	}
	fakeClient := fake.NewClientBuilder().WithObjects(existing).Build()

	var before corev1.Secret
	require.NoError(t, fakeClient.Get(t.Context(), key, &before))

	desired := &corev1.Secret{
		Name: key.Name, Namespace: key.Namespace,
		Data: map[string][]byte{"key": []byte("value")},
	}
	require.NoError(t, CreateOrUpdateSecret(t.Context(), fakeClient, desired))

	var after corev1.Secret
	require.NoError(t, fakeClient.Get(t.Context(), key, &after))
	require.Equal(t, before.ResourceVersion, after.ResourceVersion)
}

func TestCreateOrUpdateSecret_UpdatesWhenDataDiffers(t *testing.T) {
	key := types.NamespacedName{Name: "secret", Namespace: "default"}
	existing := &corev1.Secret{
		Name: key.Name, Namespace: key.Namespace,
		Data: map[string][]byte{"key": []byte("old")},
	}
	fakeClient := fake.NewClientBuilder().WithObjects(existing).Build()

	var before corev1.Secret
	require.NoError(t, fakeClient.Get(t.Context(), key, &before))

	desired := &corev1.Secret{
		Name: key.Name, Namespace: key.Namespace,
		Data: map[string][]byte{"key": []byte("new")},
	}
	require.NoError(t, CreateOrUpdateSecret(t.Context(), fakeClient, desired))

	var after corev1.Secret
	require.NoError(t, fakeClient.Get(t.Context(), key, &after))
	require.NotEqual(t, before.ResourceVersion, after.ResourceVersion)
	require.Equal(t, []byte("new"), after.Data["key"])
}
