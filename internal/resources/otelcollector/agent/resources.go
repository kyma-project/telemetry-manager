package agent

import (
	collectorconfig "github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

const (
	configMapKey            = "relay.conf"
	configHashAnnotationKey = "checksum/config"
	collectorUser           = 10001
	collectorContainerName  = "collector"
)

var (
	defaultPodAnnotations = map[string]string{
		"sidecar.istio.io/inject": "false",
	}
)

type Config struct {
	BaseName  string
	Namespace string

	DaemonSet DaemonSetConfig
}

type DaemonSetConfig struct {
	Image             string
	PriorityClassName string
}

func MakeClusterRole(name types.NamespacedName) *rbacv1.ClusterRole {
	clusterRole := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"nodes", "nodes/metrics", "nodes/stats", "services", "endpoints", "pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				NonResourceURLs: []string{"/metrics", "/metrics/cadvisor"},
				Verbs:           []string{"get"},
			},
		},
	}
	return &clusterRole
}

func MakeConfigMap(config Config, collectorConfig collectorconfig.Config) *corev1.ConfigMap {
	bytes, _ := yaml.Marshal(collectorConfig)
	confYAML := string(bytes)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.BaseName,
			Namespace: config.Namespace,
			Labels:    makeDefaultLabels(config),
		},
		Data: map[string]string{
			configMapKey: confYAML,
		},
	}
}

func MakeDaemonSet(config Config, configHash string) *appsv1.DaemonSet {
	labels := makeDefaultLabels(config)
	annotations := makePodAnnotations(configHash)

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.BaseName,
			Namespace: config.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  collectorContainerName,
							Image: config.DaemonSet.Image,
							Args:  []string{"--config=/conf/" + configMapKey},
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: config.BaseName,
										},
										Optional: pointer.Bool(true),
									},
								},
							},
							Env: []corev1.EnvVar{
								{
									Name: "MY_POD_IP",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath:  "status.podIP",
											APIVersion: "v1",
										},
									},
								},
								{
									Name: "MY_NODE_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "spec.nodeName",
										},
									},
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged:               pointer.Bool(false),
								RunAsUser:                pointer.Int64(collectorUser),
								RunAsNonRoot:             pointer.Bool(true),
								ReadOnlyRootFilesystem:   pointer.Bool(true),
								AllowPrivilegeEscalation: pointer.Bool(false),
								SeccompProfile: &corev1.SeccompProfile{
									Type: corev1.SeccompProfileTypeRuntimeDefault,
								},
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							VolumeMounts: []corev1.VolumeMount{{Name: "config", MountPath: "/conf"}},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{Path: "/", Port: intstr.IntOrString{IntVal: 13133}},
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{Path: "/", Port: intstr.IntOrString{IntVal: 13133}},
								},
							},
						},
					},
					ServiceAccountName: config.BaseName,
					PriorityClassName:  config.DaemonSet.PriorityClassName,
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: config.BaseName,
									},
									Items: []corev1.KeyToPath{{Key: configMapKey, Path: configMapKey}},
								},
							},
						},
					},
				},
			},
		},
	}
}

func makePodAnnotations(configHash string) map[string]string {
	annotations := map[string]string{
		configHashAnnotationKey: configHash,
	}
	for k, v := range defaultPodAnnotations {
		annotations[k] = v
	}
	return annotations
}

func makeDefaultLabels(config Config) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name": config.BaseName,
	}
}
