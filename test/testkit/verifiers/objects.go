package verifiers

import (
	"context"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func ShouldNotExist(ctx context.Context, k8sClient client.Client, resources ...client.Object) {
	for _, resource := range resources {
		Eventually(func(g Gomega) {
			key := types.NamespacedName{Name: resource.GetName(), Namespace: resource.GetNamespace()}
			err := k8sClient.Get(ctx, key, resource)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
	}
}

func ShouldHaveOwnerReference(ctx context.Context, k8sClient client.Client, resource client.Object, key types.NamespacedName, expectedOwnerReferenceKind, expectedOwnerReferenceName string) {
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
		ownerReferences := resource.GetOwnerReferences()
		g.Expect(ownerReferenceExists(ownerReferences, expectedOwnerReferenceKind, expectedOwnerReferenceName)).To(BeTrue())
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func ownerReferenceExists(ownerReferences []metav1.OwnerReference, expectedOwnerReferenceKind, expectedOwnerReferenceName string) bool {
	for _, ownerReference := range ownerReferences {
		if ownerReference.Kind == expectedOwnerReferenceKind && ownerReference.Name == expectedOwnerReferenceName {
			return true
		}
	}
	return false
}
