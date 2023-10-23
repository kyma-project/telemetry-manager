package k8s

import (
	"maps"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Pod struct {
	name       string
	namespace  string
	persistent bool
	labels     map[string]string
	podSpec    corev1.PodSpec
}

func NewPod(name, namespace string) *Pod {
	return &Pod{
		name:      name,
		namespace: namespace,
		labels:    make(map[string]string),
		podSpec:   SleeperPodSpec(),
	}
}

func (p *Pod) WithLabel(key, value string) *Pod {
	p.labels[key] = value
	return p
}

func (p *Pod) WithPodSpec(podSpec corev1.PodSpec) *Pod {
	p.podSpec = podSpec
	return p
}

func (p *Pod) Persistent(persistent bool) *Pod {
	p.persistent = persistent
	return p
}

func (p *Pod) K8sObject() *corev1.Pod {
	labels := p.labels
	if p.persistent {
		maps.Copy(labels, PersistentLabel)
	}

	podSpec := p.podSpec
	podSpec.RestartPolicy = corev1.RestartPolicyOnFailure

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.name,
			Namespace: p.namespace,
			Labels:    labels,
		},
		Spec: podSpec,
	}
}
