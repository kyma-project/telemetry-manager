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
