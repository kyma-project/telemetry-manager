package assert

import (
	"context"
	"fmt"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

type Resource struct {
	Object client.Object
	Name   types.NamespacedName
}

func NewResource[T client.Object](name types.NamespacedName) Resource {
	var obj T

	return Resource{
		Object: obj,
		Name:   name,
	}
}

func ResourcesExist(ctx context.Context, k8sClient client.Client, resources ...Resource) {
	for _, resource := range resources {
		Eventually(func(g Gomega) bool {
			err := suite.K8sClient.Get(ctx, resource.Name, resource.Object)
			return apierrors.IsNotFound(err)
		}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), fmt.Sprintf("resource %s of type %s should exist", resource.Name, resource.Object.GetObjectKind().GroupVersionKind().Kind))
	}
}
