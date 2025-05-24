package stdloggen

import (
	"fmt"
	"regexp"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
)

type Option func(*corev1.PodSpec)

const (
	DefaultName          = "stdloggen"
	DefaultContainerName = "stdloggen"
	DefaultImageName     = "alpine:latest"
	firstLine            = "echo 'foo bar'"
	DefaultScript        = `while true
do
` + firstLine + `
sleep 10
done`
)

func WithContainer(container string) Option {
	return func(spec *corev1.PodSpec) {
		spec.Containers[0].Name = container
	}
}

func WithScript(script string) Option {
	return func(spec *corev1.PodSpec) {
		spec.Containers[0].Command[2] = script
	}
}

func AppendLogLine(line string) Option {
	return func(spec *corev1.PodSpec) {
		regex := regexp.MustCompile(".*(" + firstLine + ").*")
		spec.Containers[0].Command[2] = regex.ReplaceAllString(spec.Containers[0].Command[2], fmt.Sprintf("echo '%s'\n%s", line, firstLine))
	}
}

func NewPod(namespace string, opts ...Option) *kitk8s.Pod {
	return kitk8s.NewPod(DefaultName, namespace).WithPodSpec(PodSpec(opts...)).WithLabel("selector", DefaultName)
}

func NewDeployment(namespace string, opts ...Option) *kitk8s.Deployment {
	return kitk8s.NewDeployment(DefaultName, namespace).WithPodSpec(PodSpec(opts...)).WithLabel("selector", DefaultName)
}

func PodSpec(opts ...Option) corev1.PodSpec {
	spec := corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:            DefaultContainerName,
				Image:           DefaultImageName,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("30Mi"),
					},
				},
				Command: []string{"/bin/sh", "-c", DefaultScript},
			},
		},
	}

	for _, opt := range opts {
		opt(&spec)
	}

	return spec
}
