package overrides

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// TODO: Move into Handler
type LogLevelReconfigurer struct {
	Atomic  zap.AtomicLevel
	Default string
}

type Handler struct {
	logLevelChanger *LogLevelReconfigurer
	client          client.Reader
	config          HandlerConfig
}

type HandlerConfig struct {
	OverridesConfigMapName types.NamespacedName
	OverridesConfigMapKey  string
}

// TODO: move to main and initialize HandlerConfig there
const overrideConfigFileName = "override-config"

func New(client client.Reader, atomicLevel zap.AtomicLevel, config HandlerConfig) *Handler {
	var m Handler
	m.logLevelChanger = NewLogReconfigurer(atomicLevel)
	m.client = client
	m.config = config
	return &m
}

func (m *Handler) LoadOverrides(ctx context.Context) (Config, error) {
	log := logf.FromContext(ctx)
	var overrideConfig Config

	config, err := m.readConfigMapOrEmpty(ctx)
	if err != nil {
		return overrideConfig, err
	}

	if len(config) == 0 {
		return overrideConfig, nil
	}

	err = yaml.Unmarshal([]byte(config), &overrideConfig)
	if err != nil {
		return overrideConfig, err
	}

	log.V(1).Info(fmt.Sprintf("Using override Config is: %+v", overrideConfig))

	return overrideConfig, nil
}

func (m *Handler) SyncLogLevel(config GlobalConfig) error {
	if config.LogLevel == "" {
		return m.logLevelChanger.setDefaultLogLevel()
	}

	return m.logLevelChanger.changeLogLevel(config.LogLevel)
}

func NewLogReconfigurer(atomicLevel zap.AtomicLevel) *LogLevelReconfigurer {
	var l LogLevelReconfigurer
	l.Atomic = atomicLevel
	l.Default = l.Atomic.String()
	return &l
}

func (l *LogLevelReconfigurer) setDefaultLogLevel() error {
	return l.changeLogLevel(l.Default)
}

func (l *LogLevelReconfigurer) changeLogLevel(logLevel string) error {
	parsedLevel, err := zapcore.ParseLevel(logLevel)
	if err != nil {
		return err
	}

	l.Atomic.SetLevel(parsedLevel)
	return nil
}

func (h *Handler) readConfigMapOrEmpty(ctx context.Context) (string, error) {
	log := logf.FromContext(ctx)
	var cm corev1.ConfigMap
	cmName := h.config.OverridesConfigMapName
	if err := h.client.Get(ctx, cmName, &cm); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info(fmt.Sprintf("Could not get  %s/%s Configmap, looks like its not present", cmName.Namespace, cmName.Name))
			return "", nil
		}
		return "", fmt.Errorf("failed to get %s/%s Configmap: %v", cmName.Namespace, cmName.Name, err)
	}
	if data, ok := cm.Data[h.config.OverridesConfigMapKey]; ok {
		return data, nil
	}
	return "", nil
}
