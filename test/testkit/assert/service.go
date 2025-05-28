package assert

import (
	"context"
	"fmt"
	"slices"

	. "github.com/onsi/gomega"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func ServiceReady(ctx context.Context, name types.NamespacedName) {
	Eventually(func(g Gomega) {
		ready, err := isServiceReady(ctx, suite.K8sClient, name)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ready).To(BeTrueBecause("Service not ready"))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func isServiceReady(ctx context.Context, k8sClient client.Client, name types.NamespacedName) (bool, error) {
	var endpointSlice discoveryv1.EndpointSlice

	err := k8sClient.Get(ctx, name, &endpointSlice)
	if err != nil {
		return false, fmt.Errorf("failed to get endpoint slice for service: %w", err)
	}

	if len(endpointSlice.Endpoints) == 0 {
		return false, nil
	}

	for _, endpoint := range endpointSlice.Endpoints {
		if len(endpoint.Addresses) == 0 {
			return false, nil
		}

		if slices.Contains(endpoint.Addresses, "") {
			return false, nil
		}
	}

	return true, nil
}
