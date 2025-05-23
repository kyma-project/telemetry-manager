package stdloggen

import (
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Option func(*corev1.PodSpec)

const (
	DefaultName          = "stdloggen"
	DefaultContainerName = "stdloggen"
	DefaultImageName     = "alpine:latest"
	DefaultScript        = `while true
do
    echo "foo bar"
    sleep 10
done`
	JSONScript = `while true
do
    echo '{"name": "a", "level": "INFO", "age": 30, "city": "Munich", "trace_id": "255c2212dd02c02ac59a923ff07aec74", "span_id": "c5c735f175ad06a6", "trace_flags": "00", "message":"a-body"}'
    echo '{"name": "b", "log.level":"WARN", "age": 30, "city": "Munich", "traceparent": "00-80e1afed08e019fc1110464cfa66635c-7a085853722dc6d2-01", "msg":"b-body"}'
    echo '{"name": "c", "age": 30, "city": "Munich", "span_id": "123456789", "body":"c-body"}'
    echo 'name=d age=30 city=Munich span_id=123456789 msg=test'
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
