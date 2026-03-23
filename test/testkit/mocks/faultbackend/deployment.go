package faultbackend

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/telemetry-manager/test/testkit"
)

func (fb *FaultBackend) buildDeployment() *appsv1.Deployment {
	labels := map[string]string{"app": fb.name}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fb.name,
			Namespace: fb.namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(fb.replicas),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "mock-backend",
							Image: testkit.MockBackendImage,
							Env:   fb.buildEnvVars(),
							Ports: []corev1.ContainerPort{
								{ContainerPort: otlpGRPCPort, Name: "grpc-otlp", Protocol: corev1.ProtocolTCP},
								{ContainerPort: otlpHTTPPort, Name: "http-otlp", Protocol: corev1.ProtocolTCP},
								{ContainerPort: httpFluentBitPushPort, Name: "http-logs", Protocol: corev1.ProtocolTCP},
							},
						},
					},
				},
			},
		},
	}
}
