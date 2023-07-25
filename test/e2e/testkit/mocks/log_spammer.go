//go:build e2e

package mocks

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/kyma-project/telemetry-manager/test/e2e/testkit"
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s"
)

type LogSpammer struct {
	name      string
	namespace string
	parser    string
}

func NewLogSpammer(name, namespace string) *LogSpammer {
	return &LogSpammer{
		name:      name + "-spammer",
		namespace: namespace,
	}
}

func (ls *LogSpammer) WithParser(parser string) *LogSpammer {
	ls.parser = parser
	return ls
}

func (ls *LogSpammer) K8sObject(labelOpts ...testkit.OptFunc) *appsv1.Deployment {
	labels := k8s.ProcessLabelOptions(labelOpts...)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ls.name,
			Namespace: ls.namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						"fluentbit.io/parser": ls.parser,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "log-spammer", Image: "alpine:3.17.2", Command: []string{"/bin/sh", "-c", `while true
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
