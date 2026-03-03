package assert

import (
	"testing"
	"time"

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

// ResourcesExist asserts that the given resources exist in the cluster.
func ResourcesExist(t *testing.T, resources ...Resource) {
	t.Helper()

	for _, resource := range resources {
		Eventually(func(g Gomega) {
			g.Expect(suite.K8sClient.Get(t.Context(), resource.Name, resource.Object)).To(Succeed())
		}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed(), "resource %s of type %T does not exist", resource.Name, resource.Object)
	}
}

// ResourcesNotExist asserts that the given resources do not exist in the cluster.
func ResourcesNotExist(t *testing.T, resources ...Resource) {
	t.Helper()

	for _, resource := range resources {
		Eventually(func(g Gomega) bool {
			err := suite.K8sClient.Get(t.Context(), resource.Name, resource.Object)
			return apierrors.IsNotFound(err)
		}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "resource %s of type %T still exists", resource.Name, resource.Object)
	}
}

// ResourcesReconciled deletes each resource and asserts that it gets recreated by the controller.
func ResourcesReconciled(t *testing.T, resources ...Resource) {
	t.Helper()

	for _, resource := range resources {
		err := suite.K8sClient.Delete(t.Context(), resource.Object)
		Expect(err).To(Succeed(), "failed to delete resource %s of type %T", resource.Name, resource.Object)

		ResourcesExist(t, resource)

		// Wait to ensure that the current reconciliation loop is finished
		// This prevents the race condition where current reconciliation could accidentally recreate an unwatched resource while testing the next one.
		time.Sleep(4 * time.Second)
	}
}
