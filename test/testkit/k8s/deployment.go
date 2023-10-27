package k8s

import (
	"maps"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Deployment struct {
	name      string
	namespace string
	replicas  int32
	labels    map[string]string
	podSpec   corev1.PodSpec
}

func NewDeployment(name, namespace string) *Deployment {
	return &Deployment{
		name:      name,
		namespace: namespace,
		replicas:  1,
		labels:    make(map[string]string),
		podSpec:   SleeperPodSpec(),
	}
}

func (d *Deployment) WithLabel(key, value string) *Deployment {
	d.labels[key] = value
	return d
}

func (d *Deployment) WithPodSpec(podSpec corev1.PodSpec) *Deployment {
	d.podSpec = podSpec
	return d
}

func (d *Deployment) K8sObject() *appsv1.Deployment {
	labels := d.labels
	maps.Copy(labels, PersistentLabel)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.name,
			Namespace: d.namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &d.replicas,
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
