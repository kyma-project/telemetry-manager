package k8s

import (
	"maps"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Job struct {
	name      string
	namespace string
	labels    map[string]string
	podSpec   corev1.PodSpec
}

func NewJob(name, namespace string) *Job {
	return &Job{
		name:      name,
		namespace: namespace,
		labels:    make(map[string]string),
		podSpec:   SleeperPodSpec(),
	}
}

func (j *Job) WithLabel(key, value string) *Job {
	j.labels[key] = value
	return j
}

func (j *Job) WithPodSpec(podSpec corev1.PodSpec) *Job {
	j.podSpec = podSpec
	return j
}

func (j *Job) K8sObject() *batchv1.Job {
	labels := j.labels
	maps.Copy(labels, PersistentLabel)

	podSpec := j.podSpec
	podSpec.RestartPolicy = corev1.RestartPolicyNever
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      j.name,
			Namespace: j.namespace,
			Labels:    j.labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: podSpec,
			},
		},
	}
}
