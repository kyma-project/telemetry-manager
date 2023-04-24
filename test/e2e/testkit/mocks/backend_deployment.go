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

const (
	replicas       = 1
	containerImage = "otel/opentelemetry-collector-contrib:0.70.0"
	nginxImage     = "nginx:1.23.3"
)

type BackendDeployment struct {
	name          string
	namespace     string
	configmapName string
}

func NewBackendDeployment(name, namespace, configmapName string) *BackendDeployment {
	return &BackendDeployment{
		name:          name,
		namespace:     namespace,
		configmapName: configmapName,
	}
}

func (d *BackendDeployment) K8sObject(labelOpts ...testkit.OptFunc) *appsv1.Deployment {
	labels := k8s.ProcessLabelOptions(labelOpts...)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.name,
			Namespace: d.namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(replicas),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "otel-collector",
							Image: containerImage,
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
							Image: nginxImage,
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
									LocalObjectReference: corev1.LocalObjectReference{Name: d.configmapName},
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
	}
}
