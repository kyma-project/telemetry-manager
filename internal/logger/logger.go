package logger

import (
	"os"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/klog/v2"
)

// New returns a logger with the given format and level.
func New(atomicLevel zap.AtomicLevel) (*zap.Logger, error) {
	logger := newWithAtomicLevel(atomicLevel)
	level := atomicLevel.Level()
	initKlog(logger, level)

	// Redirects logs those are being written using standard logging mechanism to klog
	// to avoid logs from controller-runtime being pushed to the standard logs.
	klog.CopyStandardLogTo("ERROR")

	return logger, nil
}

/*
This function creates logger structure based on given format, atomicLevel and additional cores
AtomicLevel structure allows to change level dynamically
*/
func newWithAtomicLevel(atomicLevel zap.AtomicLevel, additionalCores ...zapcore.Core) *zap.Logger {
	return newLogger(atomicLevel, additionalCores...)
}

func newLogger(levelEnabler zapcore.LevelEnabler, additionalCores ...zapcore.Core) *zap.Logger {
	encoder := getZapEncoder()

	defaultCore := zapcore.NewCore(
		encoder,
		zapcore.Lock(os.Stderr),
		levelEnabler,
	)
	cores := append(additionalCores, defaultCore)

	return zap.New(zapcore.NewTee(cores...), zap.AddCaller())
}

/*
This function initialize klog which is used in k8s/go-client
*/
func initKlog(log *zap.Logger, level zapcore.Level) {
	zaprLogger := zapr.NewLogger(log)
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
