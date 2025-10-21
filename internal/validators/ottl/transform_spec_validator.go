package ottl

import (
	"errors"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"

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

type TransformSpecValidator struct {
	// The parserCollection is intentionally not exported, because it should be initialized internally only by the constructor
	parserCollection *genericParserCollection
}

func NewTransformSpecValidator(signalType SignalType) (*TransformSpecValidator, error) {
	opts, err := newTransformParserCollectionOpts(signalType)
	if err != nil {
		return nil, err
	}

	parserCollection, err := newGenericParserCollection(opts...)
	if err != nil {
		return nil, err
	}

	return &TransformSpecValidator{parserCollection: parserCollection}, nil
}

func (v *TransformSpecValidator) Validate(transforms []telemetryv1alpha1.TransformSpec) error {
	const errorMessage = "invalid TransformSpec"

	for _, ts := range transforms {
		if err := v.parserCollection.parseStatementsWithConditions(ts.Statements, ts.Conditions); err != nil {
			return &InvalidTransformSpecError{Err: fmt.Errorf("%s: %w", errorMessage, err)}
		}

		if err := v.parserCollection.parseConditions(ts.Conditions); err != nil {
			return &InvalidTransformSpecError{Err: fmt.Errorf("%s: %w", errorMessage, err)}
		}
	}

	return nil
}

func newTransformParserCollectionOpts(signalType SignalType) ([]genericParserCollectionOption, error) {
	var opts []genericParserCollectionOption

	switch signalType {
	case SignalTypeLog:
		opts = []genericParserCollectionOption{
			withLogParser(
				ottl.CreateFactoryMap(transformprocessor.DefaultLogFunctions()...),
				ottl.WithStatementConverter(convertLogStatements),
				ottl.WithConditionConverter(convertLogConditions),
			),
		}
	case SignalTypeMetric:
		opts = []genericParserCollectionOption{
			withMetricParser(
				ottl.CreateFactoryMap(transformprocessor.DefaultMetricFunctions()...),
				ottl.WithStatementConverter(convertMetricStatements),
				ottl.WithConditionConverter(convertMetricConditions),
			),
			withDataPointParser(
				ottl.CreateFactoryMap(transformprocessor.DefaultDataPointFunctions()...),
				ottl.WithStatementConverter(convertDataPointStatements),
				ottl.WithConditionConverter(convertDataPointConditions),
			),
		}
	case SignalTypeTrace:
		opts = []genericParserCollectionOption{
			withSpanParser(
				ottl.CreateFactoryMap(transformprocessor.DefaultSpanFunctions()...),
				ottl.WithStatementConverter(convertSpanStatements),
				ottl.WithConditionConverter(convertSpanConditions),
			),
			withSpanEventParser(
				ottl.CreateFactoryMap(transformprocessor.DefaultSpanEventFunctions()...),
				ottl.WithStatementConverter(convertSpanEventStatements),
				ottl.WithConditionConverter(convertSpanEventConditions),
			),
		}
	default:
		return nil, fmt.Errorf("unexpected signal type: %s", signalType)
	}

	// Always include common context parsers
	opts = append(opts, withCommonContextsParsers()...)

	return opts, nil
}
