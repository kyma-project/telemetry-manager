package verifiers

import (
	"context"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func WebhookShouldBeHealthy(ctx context.Context, k8sClient client.Client) {
	gomega.Eventually(func(g gomega.Gomega) {
		var endpoints corev1.Endpoints
		g.Expect(k8sClient.Get(ctx, kitkyma.TelemetryOperatorWebhookServiceName, &endpoints)).To(gomega.Succeed())
		g.Expect(endpoints.Subsets).NotTo(gomega.BeEmpty())
		for _, subset := range endpoints.Subsets {
			g.Expect(subset.Addresses).NotTo(gomega.BeEmpty())
			g.Expect(subset.NotReadyAddresses).To(gomega.BeEmpty())
		}
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(gomega.Succeed())
}
