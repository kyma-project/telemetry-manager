package logger

import (
	"os"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/klog/v2"
)

type Logger struct {
	zapLogger *zap.SugaredLogger
}

// New returns a new logger with the given format and level.
func New(level zapcore.Level, atomic zap.AtomicLevel) (*Logger, error) {
	log, err := NewWithAtomicLevel(atomic)
	if err != nil {
		return nil, err
	}

	if err = InitKlog(log, level); err != nil {
		return nil, err
	}

	// Redirects logs those are being written using standard logging mechanism to klog
	// to avoid logs from controller-runtime being pushed to the standard logs.
	klog.CopyStandardLogTo("ERROR")

	return &Logger{zapLogger: log.zapLogger}, nil
}

type LogLevelReconfigurer struct {
	Atomic  zap.AtomicLevel
	Default string
}

func NewLogReconfigurer(atomic zap.AtomicLevel) *LogLevelReconfigurer {
	var l LogLevelReconfigurer
	l.Atomic = atomic
	l.Default = atomic.String()
	return &l
}

func (l *LogLevelReconfigurer) SetDefaultLogLevel() error {
	return l.ChangeLogLevel(l.Default)
}

func (l *LogLevelReconfigurer) ChangeLogLevel(logLevel string) error {
	parsedLevel, err := zapcore.ParseLevel(logLevel)
	if err != nil {
		return err
	}

	l.Atomic.SetLevel(parsedLevel)
	return nil
}

/*
This function creates logger structure based on given format, atomicLevel and additional cores
AtomicLevel structure allows to change level dynamically
*/
func NewWithAtomicLevel(atomicLevel zap.AtomicLevel, additionalCores ...zapcore.Core) (*Logger, error) {
	return new(atomicLevel, additionalCores...)
}

func new(levelEnabler zapcore.LevelEnabler, additionalCores ...zapcore.Core) (*Logger, error) {
	encoder := GetZapEncoder()

	defaultCore := zapcore.NewCore(
		encoder,
		zapcore.Lock(os.Stderr),
		levelEnabler,
	)
	cores := append(additionalCores, defaultCore)
	return &Logger{zap.New(zapcore.NewTee(cores...), zap.AddCaller()).Sugar()}, nil
}

func (l *Logger) WithContext() *zap.SugaredLogger {
	return l.zapLogger.With(zap.Namespace("context"))
}

/*
This function initialize klog which is used in k8s/go-client
*/
func InitKlog(log *Logger, level zapcore.Level) error {
	zaprLogger := zapr.NewLogger(log.WithContext().Desugar())
	zaprLogger.V((int)(level))
	klog.SetLogger(zaprLogger)
	return nil
}

func GetZapEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.MessageKey = "message"

	return zapcore.NewJSONEncoder(encoderConfig)
}
