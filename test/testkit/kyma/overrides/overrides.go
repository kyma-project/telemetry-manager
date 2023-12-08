package overrides

import (
	"strings"

	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	LogLevel = "logLevel"
)

type Level string

const (
	DEBUG Level = "debug"
	INFO  Level = "info"
	ERROR Level = "error"
)

type Overrides struct {
	level Level
}

func NewOverrides(level Level) *Overrides {
	return &Overrides{
		level: level,
	}
}

const OverridesTemplate = `override-config: |
global:
  logLevel: {{ LEVEL }}
tracing:
  paused: true
logging:
  paused: true
metrics:
  paused: true`

func (o *Overrides) K8sObject() *corev1.ConfigMap {
	var configTemplate string
	if o.level == LogLevel {
		configTemplate = LogLevel
	}

	config := strings.Replace(configTemplate, "{{ LEVEL }}", string(o.level), 1)
	data := make(map[string]string)
	data["config.yaml"] = config

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "telemetry-override-config",
			Namespace: kitkyma.SystemNamespaceName,
		},
		Data: data,
	}
}
