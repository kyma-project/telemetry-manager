package verifiers

import (
	"context"

	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func DeploymentShouldBeReady(ctx context.Context, k8sClient client.Client, name types.NamespacedName) {
	gomega.Eventually(func(g gomega.Gomega) {
		ready, err := isDeploymentReady(ctx, k8sClient, name)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(ready).To(gomega.BeTrue())
	}, periodic.Timeout, periodic.Interval).Should(gomega.Succeed())
}

func isDeploymentReady(ctx context.Context, k8sClient client.Client, name types.NamespacedName) (bool, error) {
	var deployment appsv1.Deployment
	err := k8sClient.Get(ctx, name, &deployment)
	if err != nil {
		return false, err
	}
	listOptions := client.ListOptions{
		LabelSelector: labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels),
		Namespace:     name.Namespace,
	}

	return IsPodReady(ctx, k8sClient, listOptions)
}
