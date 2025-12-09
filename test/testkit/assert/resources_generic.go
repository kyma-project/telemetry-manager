package assert

import (
	"testing"

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

func NewResource(object client.Object, name types.NamespacedName) Resource {
	return Resource{
		Object: object,
		Name:   name,
	}
}

func ResourcesExist(t *testing.T, resources ...Resource) {
	t.Helper()

	for _, resource := range resources {
		Eventually(func(g Gomega) {
			g.Expect(suite.K8sClient.Get(t.Context(), resource.Name, resource.Object)).To(Succeed())
		}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed(), "resource %s of type %T does not exist", resource.Name, resource.Object)
	}
}

func ResourcesNotExist(t *testing.T, resources ...Resource) {
	t.Helper()

	for _, resource := range resources {
		Eventually(func(g Gomega) bool {
			err := suite.K8sClient.Get(t.Context(), resource.Name, resource.Object)
			return apierrors.IsNotFound(err)
		}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "resource %s of type %T still exists", resource.Name, resource.Object)
	}
}
