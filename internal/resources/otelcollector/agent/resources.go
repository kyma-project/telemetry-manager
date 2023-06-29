package agent

import (
	collectorconfig "github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	configMapKey = "relay.conf"
)

type Config struct {
	BaseName  string
	Namespace string

	DaemonSet DaemonSetConfig
}

type DaemonSetConfig struct {
	Image string
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

func makeDefaultLabels(config Config) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name": config.BaseName,
	}
}
