package floggen

import (
	"fmt"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	DefaultName = "floggen"
)

type Option func(*corev1.PodSpec)

func WithJsonFormat() Option {
	return func(spec *corev1.PodSpec) {
		spec.Containers[0].Args = append(spec.Containers[0].Args, "-f=json")
	}
}

func NewPod(namespace string, opts ...Option) *kitk8s.Pod {
	return kitk8s.NewPod(DefaultName, namespace).WithPodSpec(PodSpec(opts...)).WithLabel("selector", DefaultName)
}

func NewDeployment(namespace string, opts ...Option) *kitk8s.Deployment {
	return kitk8s.NewDeployment(DefaultName, namespace).WithPodSpec(PodSpec(opts...)).WithLabel("selector", DefaultName)
}

func PodSpec(opts ...Option) corev1.PodSpec {
	const bytePerSecond = "10485760"

	args := []string{fmt.Sprintf("-b=%s", bytePerSecond), "-l"}

	spec := corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:            "floggen",
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

	for _, opt := range opts {
		opt(&spec)
	}

	return spec
}
