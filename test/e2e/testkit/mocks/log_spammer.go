package mocks

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type LogSpammer struct {
	name      string
	namespace string
}

func NewLogSpammer(name, namespace string) *LogSpammer {
	return &LogSpammer{
		name:      name + "-spammer",
		namespace: namespace,
	}
}

func (ls *LogSpammer) K8sObject() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ls.name,
			Namespace: ls.namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "logspammer", Image: "alpine:3.17.2", Command: []string{"/bin/sh", "-c", `while true
do
	echo "foo bar"
	sleep 10
done`}},
			},
		},
	}
}
