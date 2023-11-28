package overrides

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type LogLevelChanger interface {
	changeLogLevel(level string) error
	setDefaultLogLevel() error
}

type GlobalConfigHandler interface {
	SyncLogLevel(config GlobalConfig) error
	UpdateOverrideConfig(ctx context.Context, overrideConfigMap types.NamespacedName) (Config, error)
}

//go:generate mockery --name ConfigMapProber --filename configmap_prober.go
type ConfigMapProber interface {
	ReadConfigMapOrEmpty(ctx context.Context, name types.NamespacedName) (string, error)
}

type LogLevelReconfigurer struct {
	Atomic  zap.AtomicLevel
	Default string
}

type Config struct {
	Tracing TracingConfig `yaml:"tracing,omitempty"`
	Logging LoggingConfig `yaml:"logging,omitempty"`
	Metrics MetricConfig  `yaml:"metrics,omitempty"`
	Global  GlobalConfig  `yaml:"global,omitempty"`
}

type TracingConfig struct {
	Paused bool `yaml:"paused,omitempty"`
}

type LoggingConfig struct {
	Paused bool `yaml:"paused,omitempty"`
}

type MetricConfig struct {
	Paused bool `yaml:"paused,omitempty"`
}

type GlobalConfig struct {
	LogLevel string `yaml:"logLevel,omitempty"`
}

type Handler struct {
	logLevelChanger LogLevelChanger
	cmProber        ConfigMapProber
}

func New(loglevelChanger *LogLevelReconfigurer, cmProber ConfigMapProber) *Handler {
	var m Handler
	m.logLevelChanger = loglevelChanger
	m.cmProber = cmProber
	return &m
}

func (m *Handler) UpdateOverrideConfig(ctx context.Context, overrideConfigMap types.NamespacedName) (Config, error) {
	log := logf.FromContext(ctx)
	var overrideConfig Config

	config, err := m.cmProber.ReadConfigMapOrEmpty(ctx, overrideConfigMap)
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

func NewLogReconfigurer(atomic zap.AtomicLevel) *LogLevelReconfigurer {
	var l LogLevelReconfigurer
	l.Atomic = atomic
	l.Default = atomic.String()
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
