package backend

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/kyma-project/telemetry-manager/test/testkit"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
)

const (
	nginxImage   = "europe-docker.pkg.dev/kyma-project/prod/external/nginx:1.23.3"
	fluentDImage = "europe-docker.pkg.dev/kyma-project/prod/external/fluent/fluentd:v1.16-debian-1"
)

type collectorDeploymentBuilder struct {
	name              string
	namespace         string
	configmapName     string
	replicas          int32
	dataPath          string
	signalType        SignalType
	fluentdConfigName string
	annotations       map[string]string
}

func newCollectorDeployment(name, namespace, configmapName, dataPath string, replicas int32, signalType SignalType) *collectorDeploymentBuilder {
	return &collectorDeploymentBuilder{
		name:          name,
		namespace:     namespace,
		configmapName: configmapName,
		dataPath:      dataPath,
		replicas:      replicas,
		signalType:    signalType,
	}
}

func (d *collectorDeploymentBuilder) WithFluentdConfigName(fluentdConfigName string) *collectorDeploymentBuilder {
	d.fluentdConfigName = fluentdConfigName
	return d
}

func (d *collectorDeploymentBuilder) WithAnnotations(annotations map[string]string) *collectorDeploymentBuilder {
	d.annotations = annotations
	return d
}

func (d *collectorDeploymentBuilder) K8sObject(opts ...testkit.OptFunc) *appsv1.Deployment {
	labels := kitk8sobjects.ProcessLabelOptions(opts...)

	containers := d.containers()
	volumes := d.volumes()

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.name,
			Namespace: d.namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(d.replicas),
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

func (d *collectorDeploymentBuilder) containers() []corev1.Container {
	containers := []corev1.Container{
		{
			Name:  "otel-collector",
			Image: testkit.DefaultOTelCollectorContribImage,
			Args:  []string{"--config=/etc/collector/config.yaml"},
			SecurityContext: &corev1.SecurityContext{
				RunAsUser: ptr.To[int64](101),
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "config", MountPath: "/etc/collector"},
				{Name: "data", MountPath: d.dataPath},
			},
			Env: []corev1.EnvVar{
				{
					Name: "MY_POD_IP",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							APIVersion: "v1",
							FieldPath:  "status.podIP",
						},
					},
				},
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

	if d.signalType == SignalTypeLogsFluentBit {
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

func (d *collectorDeploymentBuilder) volumes() []corev1.Volume {
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

	if d.signalType == SignalTypeLogsFluentBit {
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
