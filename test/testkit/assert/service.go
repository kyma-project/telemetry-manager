package assert

import (
	"context"
	"fmt"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func ServiceReady(ctx context.Context, k8sClient client.Client, name types.NamespacedName) {
	Eventually(func(g Gomega) {
		ready, err := isServiceReady(ctx, k8sClient, name)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ready).To(BeTrueBecause("Service not ready"))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func isServiceReady(ctx context.Context, k8sClient client.Client, name types.NamespacedName) (bool, error) {
	var endpoint v1.Endpoints

	err := k8sClient.Get(ctx, name, &endpoint)
	if err != nil {
		return false, fmt.Errorf("failed to get endpoint for service: %w", err)
	}
	if endpoint.Subsets == nil {
		return false, nil
	}
	for _, subset := range endpoint.Subsets {
		if len(subset.Addresses) == 0 {
			return false, nil
		}
		for _, address := range subset.Addresses {
			if address.IP == "" {
				return false, nil
			}
		}
	}
	return true, nil
}
