package trafficgen

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
)

const (
	nginxImage      = "europe-docker.pkg.dev/kyma-project/prod/external/nginx:1.23.3"
	curlImage       = "europe-docker.pkg.dev/kyma-project/prod/external/curlimages/curl:7.78.0"
	sourceName      = "source"
	destinationName = "destination"
	destinationPort = int32(80)
	appLabelKey     = "app"
)

type TrafficGen struct {
	namespace string
	headers   map[string]string
}

func New(namespace string) *TrafficGen {
	return &TrafficGen{
		namespace: namespace,
		headers:   map[string]string{},
	}
}

func (t *TrafficGen) WithHeader(key, value string) *TrafficGen {
	t.headers[key] = value
	return t
}

func (t *TrafficGen) K8sObjects() []client.Object {
	return []client.Object{
		kitk8sobjects.NewPod(sourceName, t.namespace).WithPodSpec(t.sourcePodSpec()).K8sObject(),
		kitk8sobjects.NewPod(destinationName, t.namespace).WithPodSpec(destinationPodSpec()).WithLabel(appLabelKey, destinationName).K8sObject(),
		kitk8sobjects.NewService(destinationName, t.namespace).WithPort("http", destinationPort).K8sObject(kitk8sobjects.WithLabel(appLabelKey, destinationName)),
	}
}

// K8sObjects is a convenience function for the common case of no extra headers.
func K8sObjects(namespace string) []client.Object {
	return New(namespace).K8sObjects()
}

func (t *TrafficGen) sourcePodSpec() corev1.PodSpec {
	var sb strings.Builder

	for k, v := range t.headers {
		fmt.Fprintf(&sb, " -H '%s: %s'", k, v)
	}

	return corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "source",
				Image: curlImage,
				Command: []string{
					"/bin/sh",
					"-c",
					fmt.Sprintf("while true; do curl%s http://%s:%d; sleep 1; done", sb.String(), destinationName, destinationPort),
				},
			},
		},
	}
}

func destinationPodSpec() corev1.PodSpec {
	return corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "destination",
				Image: nginxImage,
				Ports: []corev1.ContainerPort{
					{
						ContainerPort: destinationPort,
						Protocol:      corev1.ProtocolTCP,
					},
				},
			},
		},
	}
}
