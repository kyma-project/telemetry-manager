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
	replicas           = 1
	otelCollectorImage = "europe-docker.pkg.dev/kyma-project/prod/tpi/otel-collector:0.75.0-6ea7fe7a"
	nginxImage         = "europe-docker.pkg.dev/kyma-project/prod/external/nginx:1.23.3"
)

type BackendDeployment struct {
	name          string
	namespace     string
	configmapName string
	dataPath      string
}

func NewBackendDeployment(name, namespace, configmapName, dataPath string) *BackendDeployment {
	return &BackendDeployment{
		name:          name,
		namespace:     namespace,
		configmapName: configmapName,
		dataPath:      dataPath,
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
							Image: otelCollectorImage,
							Args:  []string{"--config=/etc/collector/config.yaml"},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: pointer.Int64(101),
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "config", MountPath: "/etc/collector"},
								{Name: "data", MountPath: d.dataPath},
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
