package k8s

import (
	corev1 "k8s.io/api/core/v1"
)

func SleeperPodSpec() corev1.PodSpec {
	return corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "sleeper",
				Image: "busybox",
				Command: []string{
					"sh",
					"-c",
					"while true; do sleep 3600; done",
				},
			},
		},
	}
}
