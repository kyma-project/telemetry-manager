package assert

import (
	"context"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func HasOwnerReference(ctx context.Context, k8sClient client.Client, resource client.Object, key types.NamespacedName, expectedOwnerReferenceKind, expectedOwnerReferenceName string) {
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())

		ownerReferences := resource.GetOwnerReferences()
		g.Expect(ownerReferences).To(HaveLen(1))

		ownerReference := ownerReferences[0]
		g.Expect(ownerReference.Kind).To(Equal(expectedOwnerReferenceKind))
		g.Expect(ownerReference.Name).To(Equal(expectedOwnerReferenceName))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}
