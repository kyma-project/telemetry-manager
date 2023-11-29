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

type Handler struct {
	client       client.Reader
	config       HandlerConfig
	AtomicLevel  zap.AtomicLevel
	DefaultLevel string
}

type HandlerConfig struct {
	ConfigMapName types.NamespacedName
	ConfigMapKey  string
}

func New(client client.Reader, atomicLevel zap.AtomicLevel, config HandlerConfig) *Handler {
	var h Handler
	h.AtomicLevel = atomicLevel
	h.DefaultLevel = h.AtomicLevel.String()
	h.client = client
	h.config = config
	return &h
}

func (h *Handler) LoadOverrides(ctx context.Context) (Config, error) {
	log := logf.FromContext(ctx)
	var overrideConfig Config

	config, err := h.readConfigMapOrEmpty(ctx)
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

func (h *Handler) SyncLogLevel(config GlobalConfig) error {
	if config.LogLevel == "" {
		return h.setDefaultLogLevel()
	}

	return h.changeLogLevel(config.LogLevel)
}

func (h *Handler) setDefaultLogLevel() error {
	return h.changeLogLevel(h.DefaultLevel)
}

func (h *Handler) changeLogLevel(logLevel string) error {
	parsedLevel, err := zapcore.ParseLevel(logLevel)
	if err != nil {
		return err
	}

	h.AtomicLevel.SetLevel(parsedLevel)
	return nil
}

func (h *Handler) readConfigMapOrEmpty(ctx context.Context) (string, error) {
	log := logf.FromContext(ctx)
	var cm corev1.ConfigMap
	cmName := h.config.ConfigMapName
	if err := h.client.Get(ctx, cmName, &cm); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info(fmt.Sprintf("Could not get  %s/%s Configmap, looks like its not present", cmName.Namespace, cmName.Name))
			return "", nil
		}
		return "", fmt.Errorf("failed to get %s/%s Configmap: %v", cmName.Namespace, cmName.Name, err)
	}
	if data, ok := cm.Data[h.config.ConfigMapKey]; ok {
		return data, nil
	}
	return "", nil
}
