package assert

import (
	"context"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func ConfigMapHasKey(ctx context.Context, k8sClient client.Client, name types.NamespacedName, expectedKey string) {
	Eventually(func(g Gomega) {
		var configMap corev1.ConfigMap
		g.Expect(k8sClient.Get(ctx, name, &configMap)).To(Succeed())

		g.Expect(configMap.Data).Should(HaveKey(expectedKey))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

// ConfigMapConsistentlyNotHaveKey is used when the ConfigMap initially didn't contain the key, and it is expected to never contain the key
func ConfigMapConsistentlyNotHaveKey(ctx context.Context, k8sClient client.Client, name types.NamespacedName, expectedKey string) {
	Consistently(func(g Gomega) {
		var configMap corev1.ConfigMap
		g.Expect(k8sClient.Get(ctx, name, &configMap)).To(Succeed())

		g.Expect(configMap.Data).ShouldNot(HaveKey(expectedKey))
	}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(Succeed())
}

// ConfigMapEventuallyNotHaveKey is used when the ConfigMap initially contained the key, and it is expected that the key will be removed
func ConfigMapEventuallyNotHaveKey(ctx context.Context, k8sClient client.Client, name types.NamespacedName, expectedKey string) {
	Eventually(func(g Gomega) {
		var configMap corev1.ConfigMap
		g.Expect(k8sClient.Get(ctx, name, &configMap)).To(Succeed())

		g.Expect(configMap.Data).ShouldNot(HaveKey(expectedKey))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}
