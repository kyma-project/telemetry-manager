package backend

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/kyma-project/telemetry-manager/test/testkit"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
)

const (
	replicas           int32 = 1
	otelCollectorImage       = "europe-docker.pkg.dev/kyma-project/prod/tpi/otel-collector:0.95.0-0eb4394f"
	nginxImage               = "europe-docker.pkg.dev/kyma-project/prod/external/nginx:1.23.3"
	fluentDImage             = "europe-docker.pkg.dev/kyma-project/prod/external/fluent/fluentd:v1.16-debian-1"
)

type Deployment struct {
	name              string
	namespace         string
	configmapName     string
	dataPath          string
	signalType        SignalType
	fluentdConfigName string
	annotations       map[string]string
}

func NewDeployment(name, namespace, configmapName, dataPath string, signalType SignalType) *Deployment {
	return &Deployment{
		name:          name,
		namespace:     namespace,
		configmapName: configmapName,
		dataPath:      dataPath,
		signalType:    signalType,
	}
}

func (d *Deployment) WithFluentdConfigName(fluentdConfigName string) *Deployment {
	d.fluentdConfigName = fluentdConfigName
	return d
}

func (d *Deployment) WithAnnotations(annotations map[string]string) *Deployment {
	d.annotations = annotations
	return d
}
func (d *Deployment) K8sObject(opts ...testkit.OptFunc) *appsv1.Deployment {
	labels := kitk8s.ProcessLabelOptions(opts...)

	containers := d.containers()
	volumes := d.volumes()

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.name,
			Namespace: d.namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(replicas),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: d.annotations},
				Spec: corev1.PodSpec{
					Containers: containers,
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup: ptr.To[int64](101),
					},
					Volumes: volumes,
				},
			},
		},
	}
}

func (d *Deployment) containers() []corev1.Container {
	containers := []corev1.Container{
		{
			Name:  "otel-collector",
			Image: otelCollectorImage,
			Args:  []string{"--config=/etc/collector/config.yaml"},
			SecurityContext: &corev1.SecurityContext{
				RunAsUser: ptr.To[int64](101),
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
	}

	if d.signalType == SignalTypeLogs {
		containers = append(containers, corev1.Container{
			Name:  "fluentd",
			Image: fluentDImage,
			Ports: []corev1.ContainerPort{
				{ContainerPort: 9880, Name: "http-log", Protocol: corev1.ProtocolTCP},
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "fluentd-config", MountPath: "/fluentd/etc/"},
			},
		})
	}

	return containers
}

func (d *Deployment) volumes() []corev1.Volume {
	volumes := []corev1.Volume{
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
	}

	if d.signalType == SignalTypeLogs {
		volumes = append(volumes, corev1.Volume{
			Name: "fluentd-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: d.fluentdConfigName},
				},
			},
		})
	}

	return volumes
}
