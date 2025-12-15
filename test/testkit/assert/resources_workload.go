package assert

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func DeploymentReady(t *testing.T, name types.NamespacedName) {
	t.Helper()
	isReady(t, isDeploymentReady, name, "Deployment")
}

func DaemonSetReady(t *testing.T, name types.NamespacedName) {
	t.Helper()
	isReady(t, isDaemonSetReady, name, "DaemonSet")
}

func DaemonSetNotFound(t *testing.T, name types.NamespacedName) {
	t.Helper()
	Eventually(func(g Gomega) {
		_, err := isDaemonSetReady(t, name)
		g.Expect(err).To(HaveOccurred())
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func StatefulSetReady(t *testing.T, name types.NamespacedName) {
	t.Helper()
	isReady(t, isStatefulSetReady, name, "StatefulSet")
}

func JobReady(t *testing.T, name types.NamespacedName) {
	t.Helper()
	isReady(t, isJobSuccessful, name, "Job")
}

func PodReady(t *testing.T, name types.NamespacedName) {
	t.Helper()
	isReady(t, isPodReady, name, "Pod")
}

func PodsReady(t *testing.T, listOptions client.ListOptions) {
	t.Helper()
	Eventually(func(g Gomega) {
		ready, err := arePodsReady(t, listOptions)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ready).To(BeTrueBecause("Pods are not ready"))
	}, 2*periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func PodsHaveContainer(t *testing.T, listOptions client.ListOptions, containerName string) (bool, error) {
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

		for _, container := range pod.Spec.InitContainers {
			if container.Name == containerName {
				return true, nil
			}
		}
	}

	return false, nil
}

type readinessCheckFunc func(t *testing.T, name types.NamespacedName) (bool, error)

func isReady(t *testing.T, readinessCheck readinessCheckFunc, name types.NamespacedName, resourceName string) {
	t.Helper()
	Eventually(func(g Gomega) {
		t.Helper()
		ready, err := readinessCheck(t, name)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ready).To(BeTrueBecause("%s not ready: %s", resourceName, name.String()))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func isDeploymentReady(t *testing.T, name types.NamespacedName) (bool, error) {
	t.Helper()

	var deployment appsv1.Deployment

	err := suite.K8sClient.Get(t.Context(), name, &deployment)
	if err != nil {
		return false, fmt.Errorf("failed to get Deployment %s: %w", name.String(), err)
	}

	listOptions := client.ListOptions{
		LabelSelector: labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels),
		Namespace:     name.Namespace,
	}

	return arePodsReady(t, listOptions)
}

func isDaemonSetReady(t *testing.T, name types.NamespacedName) (bool, error) {
	t.Helper()

	var daemonSet appsv1.DaemonSet

	err := suite.K8sClient.Get(t.Context(), name, &daemonSet)
	if err != nil {
		return false, fmt.Errorf("failed to get DaemonSet %s: %w", name.String(), err)
	}

	listOptions := client.ListOptions{
		LabelSelector: labels.SelectorFromSet(daemonSet.Spec.Selector.MatchLabels),
		Namespace:     name.Namespace,
	}

	return arePodsReady(t, listOptions)
}

func isStatefulSetReady(t *testing.T, name types.NamespacedName) (bool, error) {
	t.Helper()

	var statefulSet appsv1.StatefulSet

	err := suite.K8sClient.Get(t.Context(), name, &statefulSet)
	if err != nil {
		return false, fmt.Errorf("failed to get StatefulSet %s: %w", name.String(), err)
	}

	listOptions := client.ListOptions{
		LabelSelector: labels.SelectorFromSet(statefulSet.Spec.Selector.MatchLabels),
		Namespace:     name.Namespace,
	}

	return arePodsReady(t, listOptions)
}

func isJobSuccessful(t *testing.T, name types.NamespacedName) (bool, error) {
	t.Helper()

	var job batchv1.Job

	err := suite.K8sClient.Get(t.Context(), name, &job)
	if err != nil {
		return false, fmt.Errorf("failed to get Job %s: %w", name.String(), err)
	}

	return job.Status.Active > 0, nil
}

func isPodReady(t *testing.T, name types.NamespacedName) (bool, error) {
	t.Helper()

	var pod corev1.Pod

	err := suite.K8sClient.Get(t.Context(), name, &pod)
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

func arePodsReady(t *testing.T, listOptions client.ListOptions) (bool, error) {
	t.Helper()

	var pods corev1.PodList

	err := suite.K8sClient.List(t.Context(), &pods, &listOptions)
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
