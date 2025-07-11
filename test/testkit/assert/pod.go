package assert

import (
	"fmt"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/testkit"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func PodReady(t testkit.T, name types.NamespacedName) {
	t.Helper()

	Eventually(func(g Gomega) {
		ready, err := isPodReady(t, suite.K8sClient, name)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ready).To(BeTrueBecause("Pod not ready: %s", name.String()))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func PodsReady(t testkit.T, listOptions client.ListOptions) {
	t.Helper()

	Eventually(func(g Gomega) {
		ready, err := arePodsReady(t, suite.K8sClient, listOptions)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ready).To(BeTrueBecause("Pods are not ready"))
	}, 2*periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func isPodReady(t testkit.T, k8sClient client.Client, name types.NamespacedName) (bool, error) {
	t.Helper()

	var pod corev1.Pod
	err := k8sClient.Get(t.Context(), name, &pod)
	if err != nil {
		return false, fmt.Errorf("failed to get Pod %s: %w", name.String(), err)
	}

	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Running == nil {
			return false, generateContainerError(pod.Name, containerStatus)
		}
	}

	return true, nil
}

func arePodsReady(t testkit.T, k8sClient client.Client, listOptions client.ListOptions) (bool, error) {
	t.Helper()

	var pods corev1.PodList
	err := k8sClient.List(t.Context(), &pods, &listOptions)
	if err != nil {
		return false, fmt.Errorf("failed to list Pods: %w", err)
	}

	for _, pod := range pods.Items {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Running == nil {
				return false, generateContainerError(pod.Name, containerStatus)
			}
		}
	}

	return true, nil
}

func generateContainerError(podName string, containerStatus corev1.ContainerStatus) error {
	var additionalInfo string
	if containerStatus.State.Waiting != nil {
		additionalInfo = fmt.Sprintf("Waiting reason: %s, message: %s", containerStatus.State.Waiting.Reason, containerStatus.State.Waiting.Message)
	} else if containerStatus.State.Terminated != nil {
		additionalInfo = fmt.Sprintf("Terminated reason: %s, message: %s", containerStatus.State.Terminated.Reason, containerStatus.State.Terminated.Message)
	}

	return fmt.Errorf("pod %s has a container %s that is not running. Additional info: %s", podName, containerStatus.Name, additionalInfo)
}

func HasContainer(t testkit.T, listOptions client.ListOptions, containerName string) (bool, error) {
	t.Helper()

	var pods corev1.PodList
	err := suite.K8sClient.List(t.Context(), &pods, &listOptions)
	if err != nil {
		return false, fmt.Errorf("failed to list Pods: %w", err)
	}

	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			if container.Name == containerName {
				return true, nil
			}
		}
	}

	return false, nil
}
