package curljob

import (
	"fmt"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CurlJob struct {
	name      string
	namespace string
	url       string
}

func New(name string, namespace string) *CurlJob {
	return &CurlJob{
		name:      name,
		namespace: namespace,
	}
}

func (c *CurlJob) SetURL(url string) {
	c.url = url
}
func (c *CurlJob) K8sObject() *v1.Job {
	return &v1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.name,
			Namespace: c.namespace,
		},
		Spec: v1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "curl",
							Image:   "radial/busyboxplus:curl",
							Command: []string{"bin/sh"},
							Args:    []string{"-c", fmt.Sprintf("for run in $(seq 1 1000); do curl %s; done", c.url)},
						},
					},
				},
			},
		},
	}
}
