package k8s

import (
	"maps"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DaemonSet struct {
	name      string
	namespace string
	labels    map[string]string
}

func NewDaemonSet(name, namespace string) *DaemonSet {
	return &DaemonSet{
		name:      name,
		namespace: namespace,
		labels:    make(map[string]string),
	}
}

func (s *DaemonSet) WithLabel(key, value string) *DaemonSet {
	s.labels[key] = value
	return s
}

func (s *DaemonSet) K8sObject() *appsv1.DaemonSet {
	labels := s.labels
	maps.Copy(labels, PersistentLabel)

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.name,
			Namespace: s.namespace,
			Labels:    s.labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: sleeperPodSpec(),
			},
		},
	}
}
