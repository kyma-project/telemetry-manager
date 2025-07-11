package assert

import (
	"fmt"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/testkit"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func DaemonSetReady(t testkit.T, name types.NamespacedName) {
	Eventually(func(g Gomega) {
		ready, err := isDaemonSetReady(t, suite.K8sClient, name)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ready).To(BeTrueBecause("DaemonSet not ready: %s", name.String()))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func DaemonSetNotFound(t testkit.T, name types.NamespacedName) {
	Eventually(func(g Gomega) {
		_, err := isDaemonSetReady(t, suite.K8sClient, name)
		g.Expect(err).To(HaveOccurred())
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func isDaemonSetReady(t testkit.T, k8sClient client.Client, name types.NamespacedName) (bool, error) {
	var daemonSet appsv1.DaemonSet

	err := k8sClient.Get(t.Context(), name, &daemonSet)
	if err != nil {
		return false, fmt.Errorf("failed to get DaemonSet %s: %w", name.String(), err)
	}

	listOptions := client.ListOptions{
		LabelSelector: labels.SelectorFromSet(daemonSet.Spec.Selector.MatchLabels),
		Namespace:     name.Namespace,
	}

	return arePodsReady(t, k8sClient, listOptions)
}
