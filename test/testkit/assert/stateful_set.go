package assert

import (
	"fmt"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/testkit"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func StatefulSetReady(t testkit.T, name types.NamespacedName) {
	t.Helper()

	Eventually(func(g Gomega) {
		ready, err := isStatefulSetReady(t, suite.K8sClient, name)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ready).To(BeTrueBecause("StatefulSet not ready: %s", name.String()))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func isStatefulSetReady(t testkit.T, k8sClient client.Client, name types.NamespacedName) (bool, error) {
	t.Helper()

	var statefulSet appsv1.StatefulSet

	err := k8sClient.Get(t.Context(), name, &statefulSet)
	if err != nil {
		return false, fmt.Errorf("failed to get StatefulSet %s: %w", name.String(), err)
	}

	listOptions := client.ListOptions{
		LabelSelector: labels.SelectorFromSet(statefulSet.Spec.Selector.MatchLabels),
		Namespace:     name.Namespace,
	}

	return arePodsReady(t, k8sClient, listOptions)
}
