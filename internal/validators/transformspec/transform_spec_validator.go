package transformspec

import (
	"errors"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type InvalidTransformSpecError struct {
	Err error
}

func (e *InvalidTransformSpecError) Error() string {
	return e.Err.Error()
}

func IsInvalidTransformSpecError(err error) bool {
	var errInvalidTransformSpec *InvalidTransformSpecError
	return errors.As(err, &errInvalidTransformSpec)
}

type SignalType string

const (
	SignalTypeLog    SignalType = "log"
	SignalTypeMetric SignalType = "metric"
	SignalTypeTrace  SignalType = "trace"
)

type Validator struct {
	// The parserCollection is intentionally not exported, because it should be initialized internally only by the constructor
	parserCollection *genericParserCollection
}

func New(signalType SignalType) (*Validator, error) {
	var (
		parserCollection *genericParserCollection
		err              error
	)

	switch signalType {
	case SignalTypeLog:
		parserCollection, err = newLogParserCollection()
	case SignalTypeMetric:
		// TODO
	case SignalTypeTrace:
		// TODO
	default:
		return nil, fmt.Errorf("unexpected signal type: %s", signalType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create TransformSpec validator: %w", err)
	}

	return &Validator{parserCollection: parserCollection}, nil
}

func newLogParserCollection() (*genericParserCollection, error) {
	telemetrySettings := component.TelemetrySettings{
		Logger: zap.New(zapcore.NewNopCore()),
	}

	functionsMap := ottl.CreateFactoryMap(transformprocessor.DefaultLogFunctions()...)

	logParserCollection, err := newGenericParserCollection(telemetrySettings, withLogParser(functionsMap))
	if err != nil {
		return nil, fmt.Errorf("failed to create log parser collection: %w", err)
	}

	return logParserCollection, nil
}

func (v *Validator) Validate(transforms []telemetryv1alpha1.TransformSpec) error {
	for _, ts := range transforms {
		if err := v.parserCollection.parseStatementsAndConditions(ts.Statements, ts.Conditions); err != nil {
			return &InvalidTransformSpecError{Err: fmt.Errorf("invalid TransformSpec: %w", err)}
		}
	}

	return nil
}
