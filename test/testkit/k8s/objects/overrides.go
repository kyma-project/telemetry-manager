package objects

import (
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
)

const (
	LogLevel = "logLevel"
)

type Level string

const (
	DEBUG Level = "debug"
	INFO  Level = "info"
)

const overridesTemplate = `global:
  logLevel: {{ LEVEL }}
tracing:
  paused: {{ PAUSED }}
logging:
  paused: {{ PAUSED }}
  collectAgentLogs: {{ AGENT_LOGS }}
metrics:
  paused: {{ PAUSED }}
telemetry:
  paused: {{ PAUSED }}`

type Overrides struct {
	paused           bool
	level            Level
	collectAgentLogs bool
}

func NewOverrides() *Overrides {
	return &Overrides{
		level:  INFO,
		paused: true,
	}
}

func (o *Overrides) WithPaused(paused bool) *Overrides {
	o.paused = paused
	return o
}

func (o *Overrides) WithLogLevel(level Level) *Overrides {
	o.level = level
	return o
}

func (o *Overrides) WithCollectAgentLogs(collectAgentLogs bool) *Overrides {
	o.collectAgentLogs = collectAgentLogs
	return o
}

func (o *Overrides) K8sObject() *corev1.ConfigMap {
	config := overridesTemplate
	config = strings.ReplaceAll(config, "{{ PAUSED }}", strconv.FormatBool(o.paused))
	config = strings.Replace(config, "{{ LEVEL }}", string(o.level), 1)
	config = strings.Replace(config, "{{ AGENT_LOGS }}", strconv.FormatBool(o.collectAgentLogs), 1)

	data := make(map[string]string)
	data["override-config"] = config

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OverrideConfigMap,
			Namespace: kitkyma.SystemNamespaceName,
		},
		Data: data,
	}
}
