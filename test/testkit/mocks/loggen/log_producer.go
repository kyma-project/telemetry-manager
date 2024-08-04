package loggen

import (
	"fmt"
	"maps"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const (
	DefaultName          = "log-producer"
	DefaultContainerName = "log-producer"
)

type Load int

const (
	LoadLow Load = iota
	LoadHigh
)

type LogProducer struct {
	name        string
	namespace   string
	annotations map[string]string
	labels      map[string]string
	replicas    int32
	load        Load
	useJSON     bool
}

func New(namespace string) *LogProducer {
	return &LogProducer{
		name:      DefaultName,
		namespace: namespace,
		replicas:  1,
		load:      LoadLow,
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

func (lp *LogProducer) WithReplicas(replicas int32) *LogProducer {
	lp.replicas = replicas
	return lp
}

func (lp *LogProducer) WithLoad(load Load) *LogProducer {
	lp.load = load
	return lp
}

func (lp *LogProducer) WithUseJSON() *LogProducer {
	lp.useJSON = true
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
			Replicas: ptr.To(lp.replicas),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: lp.annotations,
				},
				Spec: lp.podSpec(),
			},
		},
	}
}

func (lp *LogProducer) podSpec() corev1.PodSpec {
	if lp.load == LoadLow {
		return lp.alpineSpec()
	}
	return lp.flogSpec()
}

func (lp *LogProducer) alpineSpec() corev1.PodSpec {
	logCmd := `while true
do
	echo "foo bar"
	sleep 500
done`
	if lp.useJSON {
		logCmd = `while true
do
	echo '{"name": "John Doe", "age": 30, "city": "Munich"}'
	sleep 500
done`
	}

	return corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:    DefaultContainerName,
				Image:   "alpine:3.17.2",
				Command: []string{"/bin/sh", "-c", logCmd}},
		},
	}
}

func (lp *LogProducer) flogSpec() corev1.PodSpec {
	const bytePerSecond = "10485760"
	args := []string{fmt.Sprintf("-b=%s", bytePerSecond), "-l"}
	if lp.useJSON {
		args = append(args, "-f=json")
	}
	return corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:            DefaultContainerName,
				Image:           "mingrammer/flog",
				Args:            args,
				ImagePullPolicy: corev1.PullAlways,
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("200Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
					},
				},
			},
		},
	}
}
