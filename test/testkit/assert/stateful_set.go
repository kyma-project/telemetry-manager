package assert

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

func StatefulSetReady(ctx context.Context, k8sClient client.Client, name types.NamespacedName) {
	Eventually(func(g Gomega) {
		ready, err := isStatefulSetReady(ctx, k8sClient, name)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ready).To(BeTrueBecause("statefulSet not ready"))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func isStatefulSetReady(ctx context.Context, k8sClient client.Client, name types.NamespacedName) (bool, error) {
	var statefulSet appsv1.StatefulSet

	err := k8sClient.Get(ctx, name, &statefulSet)
	if err != nil {
		return false, fmt.Errorf("failed to get statefulSet: %w", err)
	}

	listOptions := client.ListOptions{
		LabelSelector: labels.SelectorFromSet(statefulSet.Spec.Selector.MatchLabels),
		Namespace:     name.Namespace,
	}

	return arePodsReady(ctx, k8sClient, listOptions)
}
