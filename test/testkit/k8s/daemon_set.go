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
	podSpec   corev1.PodSpec
}

func NewDaemonSet(name, namespace string) *DaemonSet {
	return &DaemonSet{
		name:      name,
		namespace: namespace,
		labels:    make(map[string]string),
		podSpec:   SleeperPodSpec(),
	}
}

func (d *DaemonSet) WithLabel(key, value string) *DaemonSet {
	d.labels[key] = value
	return d
}

func (d *DaemonSet) WithPodSpec(podSpec corev1.PodSpec) *DaemonSet {
	d.podSpec = podSpec
	return d
}

func (d *DaemonSet) K8sObject() *appsv1.DaemonSet {
	labels := d.labels
	maps.Copy(labels, PersistentLabel)

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.name,
			Namespace: d.namespace,
			Labels:    d.labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: d.podSpec,
			},
		},
	}
}
