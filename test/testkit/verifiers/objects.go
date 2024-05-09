package verifiers

import (
	"context"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

func ShouldHaveCorrectOwnerReference(ctx context.Context, k8sClient client.Client, resource client.Object, key types.NamespacedName, expectedOwnerReferenceKind, expectedOwnerReferenceName string) {
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())

		ownerReferences := resource.GetOwnerReferences()
		g.Expect(len(ownerReferences)).To(Equal(1))

		ownerReference := ownerReferences[0]
		g.Expect(ownerReference.Kind).To(Equal(expectedOwnerReferenceKind))
		g.Expect(ownerReference.Name).To(Equal(expectedOwnerReferenceName))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}
