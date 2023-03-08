//go:build e2e

package e2e

import (
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func deployMockTraceReceiver(c client.Client) error {
	namespace := "mocks"
	labels := map[string]string{
		"app": "trace-receiver",
	}

	ctx := context.Background()
	if err := c.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}); err != nil {
		return fmt.Errorf("failed to create namespace: %v", err)
	}

	if err := c.Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "trace-receiver-config",
			Namespace: namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			"config.yaml": `receivers:
  otlp:
    protocols:
      grpc: {}
      http: {}
exporters:
  file:
    path: /traces/spans.json
  logging:
    loglevel: debug
service:
  pipelines:
    traces:
      receivers:
      - otlp
      exporters:
      - file
      - logging`,
		},
	}); err != nil {
		return fmt.Errorf("failed to create configmap: %v", err)
	}

	if err := c.Create(ctx, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "trace-receiver",
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "otel-collector",
							Image: "otel/opentelemetry-collector-contrib:0.70.0",
							Args:  []string{"--config=/etc/collector/config.yaml"},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: pointer.Int64(101),
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "config", MountPath: "/etc/collector"},
								{Name: "data", MountPath: "/traces"},
							},
						},
						{
							Name:  "web",
							Image: "nginx:1.23.3",
							VolumeMounts: []corev1.VolumeMount{
								{Name: "data", MountPath: "/usr/share/nginx/html"},
							},
						},
					},
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup: pointer.Int64(101),
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: "trace-receiver-config"},
								},
							},
						},
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}); err != nil {
		return fmt.Errorf("failed to create deployment: %v", err)
	}

	if err := c.Create(ctx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "trace-receiver",
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "grpc-otlp",
					Protocol:   corev1.ProtocolTCP,
					Port:       4317,
					TargetPort: intstr.FromInt(4317),
				},
				{
					Name:       "http-otlp",
					Protocol:   corev1.ProtocolTCP,
					Port:       4318,
					TargetPort: intstr.FromInt(4318),
				},
				{
					Name:       "export-http",
					Protocol:   corev1.ProtocolTCP,
					Port:       8080,
					TargetPort: intstr.FromInt(80),
				},
			},
			Selector: labels,
			Type:     corev1.ServiceTypeLoadBalancer,
		},
	}); err != nil {
		return fmt.Errorf("failed to create service: %v", err)
	}

	return nil
}
