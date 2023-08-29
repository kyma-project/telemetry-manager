package logproducer

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/kyma-project/telemetry-manager/test/testkit"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s"
)

type LogProducer struct {
	name      string
	namespace string
	parser    string
}

func New(namespace string) *LogProducer {
	return &LogProducer{
		name:      "log-producer",
		namespace: namespace,
	}
}

func (lp *LogProducer) WithParser(parser string) *LogProducer {
	lp.parser = parser
	return lp
}

func (lp *LogProducer) K8sObject(labelOpts ...testkit.OptFunc) *appsv1.Deployment {
	labels := k8s.ProcessLabelOptions(labelOpts...)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      lp.name,
			Namespace: lp.namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						"fluentbit.io/parser": lp.parser,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: lp.name, Image: "alpine:3.17.2", Command: []string{"/bin/sh", "-c", `while true
do
	echo "foo bar"
	sleep 10
done`}},
					},
				},
			},
		},
	}
}
