package assert

import (
	"context"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func WebhookHealthy(ctx context.Context, k8sClient client.Client) {
	Eventually(func(g Gomega) {
		var endpoints corev1.Endpoints
		g.Expect(k8sClient.Get(ctx, kitkyma.TelemetryManagerWebhookServiceName, &endpoints)).To(Succeed())
		g.Expect(endpoints.Subsets).NotTo(BeEmpty())
		for _, subset := range endpoints.Subsets {
			g.Expect(subset.Addresses).NotTo(BeEmpty())
			g.Expect(subset.NotReadyAddresses).To(BeEmpty())
		}
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}
