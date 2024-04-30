package loggen

import (
	"maps"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const (
	DefaultName = "log-producer"
)

type LogProducer struct {
	name        string
	namespace   string
	annotations map[string]string
	labels      map[string]string
}

func New(namespace string) *LogProducer {
	return &LogProducer{
		name:      DefaultName,
		namespace: namespace,
	}
}

func (lp *LogProducer) WithAnnotations(annotations map[string]string) *LogProducer {
	lp.annotations = annotations
	return lp
}

func (lp *LogProducer) WithLabels(labels map[string]string) *LogProducer {
	lp.labels = labels
	return lp
}

func (lp *LogProducer) K8sObject() *appsv1.Deployment {
	labels := map[string]string{"app": lp.name}
	if lp.labels != nil {
		maps.Copy(labels, lp.labels)
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      lp.name,
			Namespace: lp.namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: lp.annotations,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: lp.name, Image: "alpine:3.17.2", Command: []string{"/bin/sh", "-c", `while true
do
	echo "foo bar"
	sleep 500
done`}},
					},
				},
			},
		},
	}
}
