package fluentbit

import (
	"fmt"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"github.com/kyma-project/telemetry-manager/internal/fluentbit/ports"
)

const checksumAnnotationKey = "checksum/logpipeline-config"
const istioExcludeInboundPorts = "traffic.sidecar.istio.io/excludeInboundPorts"

type DaemonSetConfig struct {
	FluentBitImage              string
	FluentBitConfigPrepperImage string
	ExporterImage               string
	PriorityClassName           string
	CPULimit                    resource.Quantity
	MemoryLimit                 resource.Quantity
	CPURequest                  resource.Quantity
	MemoryRequest               resource.Quantity
}

func MakeDaemonSet(name types.NamespacedName, checksum string, dsConfig DaemonSetConfig) *appsv1.DaemonSet {
	resourcesFluentBit := corev1.ResourceRequirements{
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    dsConfig.CPURequest,
			corev1.ResourceMemory: dsConfig.MemoryRequest,
		},
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    dsConfig.CPULimit,
			corev1.ResourceMemory: dsConfig.MemoryLimit,
		},
	}

	resourcesExporter := corev1.ResourceRequirements{
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceMemory: resource.MustParse("5Mi"),
		},
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("50Mi"),
		},
	}

	annotations := make(map[string]string)
	annotations[checksumAnnotationKey] = checksum
	annotations[istioExcludeInboundPorts] = fmt.Sprintf("%v,%v", ports.HTTP, ports.ExporterMetrics)
	return &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    Labels(),
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: Labels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      Labels(),
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: name.Name,
					PriorityClassName:  dsConfig.PriorityClassName,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot:   ptr.To(false),
						SeccompProfile: &corev1.SeccompProfile{Type: "RuntimeDefault"},
					},
					Containers: []corev1.Container{
						{
							Name: "fluent-bit",
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities: &corev1.Capabilities{
									Add:  []corev1.Capability{"FOWNER"},
									Drop: []corev1.Capability{"ALL"},
								},
								Privileged:             ptr.To(false),
								ReadOnlyRootFilesystem: ptr.To(true),
							},
							Image:           dsConfig.FluentBitImage,
							ImagePullPolicy: "IfNotPresent",
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%s-env", name.Name)},
										Optional:             ptr.To(true),
									},
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: ports.HTTP,
									Protocol:      "TCP",
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/",
										Port: intstr.FromString("http"),
									},
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/api/v1/health",
										Port: intstr.FromString("http"),
									},
								},
							},
							Resources: resourcesFluentBit,
							VolumeMounts: []corev1.VolumeMount{
								{MountPath: "/fluent-bit/etc", Name: "shared-fluent-bit-config"},
								{MountPath: "/fluent-bit/etc/fluent-bit.conf", Name: "config", SubPath: "fluent-bit.conf"},
								{MountPath: "/fluent-bit/etc/dynamic/", Name: "dynamic-config"},
								{MountPath: "/fluent-bit/etc/dynamic-parsers/", Name: "dynamic-parsers-config"},
								{MountPath: "/fluent-bit/etc/custom_parsers.conf", Name: "config", SubPath: "custom_parsers.conf"},
								{MountPath: "/fluent-bit/scripts/filter-script.lua", Name: "luascripts", SubPath: "filter-script.lua"},
								{MountPath: "/var/log", Name: "varlog", ReadOnly: true},
								{MountPath: "/data", Name: "varfluentbit"},
								{MountPath: "/files", Name: "dynamic-files"},
								{MountPath: "/fluent-bit/etc/output-tls-config/", Name: "output-tls-config", ReadOnly: true},
							},
						},
						{
							Name:      "exporter",
							Image:     dsConfig.ExporterImage,
							Resources: resourcesExporter,
							Args: []string{
								"--storage-path=/data/flb-storage/",
								"--metric-name=telemetry_fsbuffer_usage_bytes",
							},
							WorkingDir: "",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http-metrics",
									ContainerPort: ports.ExporterMetrics,
									Protocol:      "TCP",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								Privileged:               ptr.To(false),
								ReadOnlyRootFilesystem:   ptr.To(true),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "varfluentbit", MountPath: "/data"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: name.Name},
								},
							},
						},
						{
							Name: "luascripts",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%s-luascripts", name.Name)},
								},
							},
						},
						{
							Name: "varlog",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{Path: "/var/log"},
							},
						},
						{
							Name: "shared-fluent-bit-config",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "dynamic-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%s-sections", name.Name)},
									Optional:             ptr.To(true),
								},
							},
						},
						{
							Name: "dynamic-parsers-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%s-parsers", name.Name)},
									Optional:             ptr.To(true),
								},
							},
						},
						{
							Name: "dynamic-files",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%s-files", name.Name)},
									Optional:             ptr.To(true),
								},
							},
						},
						{
							Name: "varfluentbit",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{Path: fmt.Sprintf("/var/%s", name.Name)},
							},
						},
						{
							Name: "output-tls-config",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: fmt.Sprintf("%s-output-tls-config", name.Name),
								},
							},
						},
					},
				},
			},
		},
	}
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
				Resources: []string{"namespaces", "pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
	return &clusterRole
}

func MakeMetricsService(name types.NamespacedName) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-metrics", name.Name),
			Namespace: name.Namespace,
			Labels:    Labels(),
			Annotations: map[string]string{
				"prometheus.io/scrape": "true",
				"prometheus.io/port":   strconv.Itoa(ports.HTTP),
				"prometheus.io/scheme": "http",
				"prometheus.io/path":   "/api/v1/metrics/prometheus",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   "TCP",
					Port:       int32(ports.HTTP),
					TargetPort: intstr.FromString("http"),
				},
			},
			Selector: Labels(),
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
}

func MakeExporterMetricsService(name types.NamespacedName) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-exporter-metrics", name.Name),
			Namespace: name.Namespace,
			Labels:    Labels(),
			Annotations: map[string]string{
				"prometheus.io/scrape": "true",
				"prometheus.io/port":   strconv.Itoa(ports.ExporterMetrics),
				"prometheus.io/scheme": "http",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http-metrics",
					Protocol:   "TCP",
					Port:       int32(ports.ExporterMetrics),
					TargetPort: intstr.FromString("http-metrics"),
				},
			},
			Selector: Labels(),
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
}

func MakeConfigMap(name types.NamespacedName, includeSections bool) *corev1.ConfigMap {
	parserConfig := `
[PARSER]
    Name docker_no_time
    Format json
    Time_Keep Off
    Time_Key time
    Time_Format %Y-%m-%dT%H:%M:%S.%L
`

	fluentBitConfig := `
[SERVICE]
    Daemon Off
    Flush 1
    Log_Level warn
    Parsers_File custom_parsers.conf
    Parsers_File dynamic-parsers/parsers.conf
    HTTP_Server On
    HTTP_Listen 0.0.0.0
    HTTP_Port {{ HTTP_PORT }}
    storage.path /data/flb-storage/
    storage.metrics on

[INPUT]
    Name tail
    Path /null.log
    Tag null.*
    Alias null-tail

[OUTPUT]
    Name null
    Match null.*
    Alias null-null

`
	fluentBitConfig = strings.Replace(fluentBitConfig, "{{ HTTP_PORT }}", strconv.Itoa(ports.HTTP), 1)
	if includeSections {
		fluentBitConfig = fluentBitConfig + "@INCLUDE dynamic/*.conf" + "\n"
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    Labels(),
		},
		Data: map[string]string{
			"custom_parsers.conf": parserConfig,
			"fluent-bit.conf":     fluentBitConfig,
		},
	}
}

func MakeParserConfigmap(name types.NamespacedName) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    Labels(),
		},
		Data: map[string]string{"parsers.conf": ""},
	}
}

func MakeLuaConfigMap(name types.NamespacedName) *corev1.ConfigMap {
	//nolint:dupword // Ignore lua syntax code duplications.
	luaFilter := `
function kubernetes_map_keys(tag, timestamp, record)
  if record.kubernetes == nil then
    return 0
  end
  map_keys(record.kubernetes.annotations)
  map_keys(record.kubernetes.labels)
  return 1, timestamp, record
end
function map_keys(table)
  if table == nil then
    return
  end
  local new_table = {}
  local changed_keys = {}
  for key, val in pairs(table) do
    local mapped_key = string.gsub(key, "[%/%.]", "_")
    if mapped_key ~= key then
      new_table[mapped_key] = val
      changed_keys[key] = true
    end
  end
  for key in pairs(changed_keys) do
    table[key] = nil
  end
  for key, val in pairs(new_table) do
    table[key] = val
  end
end
`

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    Labels(),
		},
		Data: map[string]string{"filter-script.lua": luaFilter},
	}
}

func Labels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":     "fluent-bit",
		"app.kubernetes.io/instance": "telemetry",
	}
}
