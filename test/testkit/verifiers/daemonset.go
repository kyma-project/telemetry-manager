package verifiers

import (
	"context"
	"fmt"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func DaemonSetShouldBeReady(ctx context.Context, k8sClient client.Client, name types.NamespacedName) {
	Eventually(func(g Gomega) {
		ready, err := isDaemonSetReady(ctx, k8sClient, name)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ready).To(BeTrue())
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func isDaemonSetReady(ctx context.Context, k8sClient client.Client, name types.NamespacedName) (bool, error) {
	var daemonSet appsv1.DaemonSet
	err := k8sClient.Get(ctx, name, &daemonSet)
	if err != nil {
		return false, fmt.Errorf("failed to get daemonset: %w", err)
	}
	listOptions := client.ListOptions{
		LabelSelector: labels.SelectorFromSet(daemonSet.Spec.Selector.MatchLabels),
		Namespace:     name.Namespace,
	}
	return IsPodReady(ctx, k8sClient, listOptions)
}
