package trafficgen

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
)

const (
	nginxImage      = "europe-docker.pkg.dev/kyma-project/prod/external/nginx:1.23.3"
	curlImage       = "europe-docker.pkg.dev/kyma-project/prod/external/curlimages/curl:7.78.0"
	sourceName      = "source"
	destinationName = "destination"
	appLabelKey     = "app"
)

func sourcePodSpec() corev1.PodSpec {
	return corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "source",
				Image: curlImage,
				Command: []string{
					"/bin/sh",
					"-c",
					"while true; do curl http://destination:80; sleep 1; done",
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
						ContainerPort: 80,
						Protocol:      corev1.ProtocolTCP,
					},
				},
			},
		},
	}
}

func K8sObjects(namespace string) []client.Object {
	return []client.Object{
		kitk8s.NewPod(sourceName, namespace).WithPodSpec(sourcePodSpec()).K8sObject(),
		kitk8s.NewPod(destinationName, namespace).WithPodSpec(destinationPodSpec()).WithLabel(appLabelKey, destinationName).K8sObject(),
		kitk8s.NewService(destinationName, namespace).WithPort("http", 80).K8sObject(kitk8s.WithLabel(appLabelKey, destinationName)),
	}
}
