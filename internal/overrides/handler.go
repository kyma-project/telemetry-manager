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
	atomicLevel  zap.AtomicLevel
	defaultLevel zapcore.Level
}

type HandlerConfig struct {
	ConfigMapName types.NamespacedName
	ConfigMapKey  string
}

func New(client client.Reader, atomicLevel zap.AtomicLevel, config HandlerConfig) *Handler {
	return &Handler{
		atomicLevel:  atomicLevel,
		defaultLevel: atomicLevel.Level(),
		client:       client,
		config:       config,
	}
}

func (h *Handler) LoadOverrides(ctx context.Context) (*Config, error) {
	overrideConfig, err := h.loadOverrides(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load overrides config: %w", err)
	}

	if err := h.syncLogLevel(ctx, overrideConfig.Global); err != nil {
		return nil, fmt.Errorf("failed to sync log level: %w", err)
	}

	return overrideConfig, nil
}

func (h *Handler) loadOverrides(ctx context.Context) (*Config, error) {
	var overrideConfig Config

	config, err := h.readConfigMapOrEmpty(ctx)
	if err != nil {
		return &overrideConfig, err
	}

	if len(config) == 0 {
		return &overrideConfig, nil
	}

	err = yaml.Unmarshal([]byte(config), &overrideConfig)
	if err != nil {
		return &overrideConfig, err
	}

	logf.FromContext(ctx).V(1).Info("Using overrides: %+v", overrideConfig)

	return &overrideConfig, nil
}

func (h *Handler) readConfigMapOrEmpty(ctx context.Context) (string, error) {
	var cm corev1.ConfigMap
	cmName := h.config.ConfigMapName
	if err := h.client.Get(ctx, cmName, &cm); err != nil {
		if apierrors.IsNotFound(err) {
			logf.FromContext(ctx).V(1).Info("Could not find overrides configmap",
				"name", cmName.Name,
				"namespace", cmName.Namespace)
			return "", nil
		}
		return "", fmt.Errorf("failed to get overrides configmapp: %w", err)
	}
	if data, ok := cm.Data[h.config.ConfigMapKey]; ok {
		return data, nil
	}
	return "", nil
}

func (h *Handler) syncLogLevel(ctx context.Context, config GlobalConfig) error {
	var newLogLevel zapcore.Level
	if config.LogLevel == "" {
		newLogLevel = h.defaultLevel
	} else {
		var err error
		newLogLevel, err = zapcore.ParseLevel(config.LogLevel)
		if err != nil {
			return fmt.Errorf("failed to parse zap level: %w", err)
		}
	}

	return h.changeLogLevel(ctx, newLogLevel)
}

func (h *Handler) changeLogLevel(ctx context.Context, newLevel zapcore.Level) error {
	oldLevel := h.atomicLevel.Level()

	logf.FromContext(ctx).V(1).Info("Changing log level",
		"old", oldLevel,
		"new", newLevel)

	h.atomicLevel.SetLevel(newLevel)
	return nil
}
