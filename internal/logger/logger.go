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
func New(atomicLevel zap.AtomicLevel) (*Logger, error) {
	log := newWithAtomicLevel(atomicLevel)
	level := atomicLevel.Level()
	initKlog(log, level)

	// Redirects logs those are being written using standard logging mechanism to klog
	// to avoid logs from controller-runtime being pushed to the standard logs.
	klog.CopyStandardLogTo("ERROR")

	return &Logger{zapLogger: log.zapLogger}, nil
}

func (l *Logger) WithContext() *zap.SugaredLogger {
	return l.zapLogger.With(zap.Namespace("context"))
}

/*
This function creates logger structure based on given format, atomicLevel and additional cores
AtomicLevel structure allows to change level dynamically
*/
func newWithAtomicLevel(atomicLevel zap.AtomicLevel, additionalCores ...zapcore.Core) *Logger {
	return new(atomicLevel, additionalCores...)
}

func new(levelEnabler zapcore.LevelEnabler, additionalCores ...zapcore.Core) *Logger {
	encoder := getZapEncoder()

	defaultCore := zapcore.NewCore(
		encoder,
		zapcore.Lock(os.Stderr),
		levelEnabler,
	)
	cores := append(additionalCores, defaultCore)
	return &Logger{zap.New(zapcore.NewTee(cores...), zap.AddCaller()).Sugar()}
}

/*
This function initialize klog which is used in k8s/go-client
*/
func initKlog(log *Logger, level zapcore.Level) {
	zaprLogger := zapr.NewLogger(log.WithContext().Desugar())
	zaprLogger.V((int)(level))
	klog.SetLogger(zaprLogger)
}

func getZapEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.MessageKey = "message"

	return zapcore.NewJSONEncoder(encoderConfig)
}
