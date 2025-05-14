package assert

import (
	"context"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

type Resource struct {
	Object client.Object
	Name   types.NamespacedName
}

func NewResource(object client.Object, name types.NamespacedName) Resource {
	return Resource{
		Object: object,
		Name:   name,
	}
}

func ResourcesExist(ctx context.Context, k8sClient client.Client, resources ...Resource) {
	for _, resource := range resources {
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, resource.Name, resource.Object)).To(Succeed())
		}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed(), "resource %s of type %T does not exist", resource.Name, resource.Object)
	}
}

func ResourcesNotExist(ctx context.Context, k8sClient client.Client, resources ...Resource) {
	for _, resource := range resources {
		Eventually(func(g Gomega) bool {
			err := k8sClient.Get(ctx, resource.Name, resource.Object)
			return apierrors.IsNotFound(err)
		}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "resource %s of type %T still exists", resource.Name, resource.Object)
	}
}
