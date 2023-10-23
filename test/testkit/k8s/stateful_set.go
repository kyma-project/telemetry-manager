package k8s

import (
	"maps"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StatefulSet struct {
	name      string
	namespace string
	replicas  int32
	labels    map[string]string
	podSpec   corev1.PodSpec
}

func NewStatefulSet(name, namespace string) *StatefulSet {
	return &StatefulSet{
		name:      name,
		namespace: namespace,
		replicas:  1,
		labels:    make(map[string]string),
		podSpec:   SleeperPodSpec(),
	}
}

func (s *StatefulSet) WithLabel(key, value string) *StatefulSet {
	s.labels[key] = value
	return s
}

func (s *StatefulSet) WithPodSpec(podSpec corev1.PodSpec) *StatefulSet {
	s.podSpec = podSpec
	return s
}

func (s *StatefulSet) K8sObject() *appsv1.StatefulSet {
	labels := s.labels
	maps.Copy(labels, PersistentLabel)

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.name,
			Namespace: s.namespace,
			Labels:    s.labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &s.replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: s.podSpec,
			},
		},
	}
}
