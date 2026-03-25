package telemetry

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// isPodFrom checks whether a pod belongs to one of the given workloads by verifying
// that the pod name starts with one of the provided workload name prefixes.
func isPodFrom(pod *corev1.Pod, workloadNames ...string) bool {
	for _, name := range workloadNames {
		if strings.HasPrefix(pod.Name, name) {
			return true
		}
	}

	return false
}
