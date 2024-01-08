package verifiers

import (
	"context"

	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func WebhookShouldBeHealthy(ctx context.Context, k8sClient client.Client) {
	Eventually(func(g Gomega) {
		var endpoints corev1.Endpoints
		g.Expect(k8sClient.Get(ctx, kitkyma.TelemetryOperatorWebhookServiceName, &endpoints)).To(Succeed())
		g.Expect(endpoints.Subsets).NotTo(BeEmpty())
		for _, subset := range endpoints.Subsets {
			g.Expect(subset.Addresses).NotTo(BeEmpty())
			g.Expect(subset.NotReadyAddresses).To(BeEmpty())
		}
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func PipelineReconciliationShouldBeDisabled(ctx context.Context, k8sClient client.Client, configMapName string, labelKey string) {
	key := types.NamespacedName{
		Name:      configMapName,
		Namespace: kitkyma.SystemNamespaceName,
	}
	var configMap corev1.ConfigMap
	Expect(k8sClient.Get(ctx, key, &configMap)).To(Succeed())

	delete(configMap.ObjectMeta.Labels, labelKey)
	Expect(k8sClient.Update(ctx, &configMap)).To(Succeed())

	// The deleted label should not be restored, since the reconciliation is disabled by the overrides configmap
	Consistently(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, key, &configMap)).To(Succeed())
		g.Expect(configMap.ObjectMeta.Labels[labelKey]).To(BeZero())
	}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func TelemetryReconciliationShouldBeDisabled(ctx context.Context, k8sClient client.Client, webhookName string, labelKey string) {
	key := types.NamespacedName{
		Name: webhookName,
	}
	var validatingWebhookConfiguration admissionregistrationv1.ValidatingWebhookConfiguration
	Expect(k8sClient.Get(ctx, key, &validatingWebhookConfiguration)).To(Succeed())

	delete(validatingWebhookConfiguration.ObjectMeta.Labels, labelKey)
	Expect(k8sClient.Update(ctx, &validatingWebhookConfiguration)).To(Succeed())

	// The deleted label should not be restored, since the reconciliation is disabled by the overrides configmap
	Consistently(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, key, &validatingWebhookConfiguration)).To(Succeed())
		g.Expect(validatingWebhookConfiguration.ObjectMeta.Labels[labelKey]).To(BeZero())
	}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(Succeed())
}
