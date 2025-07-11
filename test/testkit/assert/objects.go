package assert

import (
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/testkit"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func HasOwnerReference(t testkit.T, resource client.Object, key types.NamespacedName, expectedOwnerReferenceKind, expectedOwnerReferenceName string) {
	t.Helper()

	Eventually(func(g Gomega) {
		g.Expect(suite.K8sClient.Get(t.Context(), key, resource)).To(Succeed())
		ownerReferences := resource.GetOwnerReferences()
		g.Expect(ownerReferences).Should(ContainElement(SatisfyAll(
			HaveField("Kind", expectedOwnerReferenceKind),
			HaveField("Name", expectedOwnerReferenceName),
		)))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}
