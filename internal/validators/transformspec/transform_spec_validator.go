package transformspec

import (
	"errors"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
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
		parserCollection, err = newMetricParserCollection()
	case SignalTypeTrace:
		parserCollection, err = newTraceParserCollection()
	default:
		return nil, fmt.Errorf("unexpected signal type: %s", signalType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create TransformSpec validator: %w", err)
	}

	return &Validator{parserCollection: parserCollection}, nil
}

func (v *Validator) Validate(transforms []telemetryv1alpha1.TransformSpec) error {
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

func newLogParserCollection() (*genericParserCollection, error) {
	telemetrySettings := component.TelemetrySettings{
		Logger: zap.New(zapcore.NewNopCore()),
	}

	parserCollectionOpts := []genericParserCollectionOption{
		withLogParser(
			ottl.CreateFactoryMap(transformprocessor.DefaultLogFunctions()...),
			ottl.WithStatementConverter(convertLogStatements),
			ottl.WithConditionConverter(convertLogConditions),
		),
	}

	parserCollectionOpts = append(parserCollectionOpts, withCommonContextsParsers()...)

	logParserCollection, err := newGenericParserCollection(telemetrySettings, parserCollectionOpts...)

	if err != nil {
		return nil, fmt.Errorf("failed to create log parser collection: %w", err)
	}

	return logParserCollection, nil
}

func newMetricParserCollection() (*genericParserCollection, error) {
	telemetrySettings := component.TelemetrySettings{
		Logger: zap.New(zapcore.NewNopCore()),
	}

	parserCollectionOpts := []genericParserCollectionOption{
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

	parserCollectionOpts = append(parserCollectionOpts, withCommonContextsParsers()...)

	metricParserCollection, err := newGenericParserCollection(telemetrySettings, parserCollectionOpts...)

	if err != nil {
		return nil, fmt.Errorf("failed to create metric parser collection: %w", err)
	}

	return metricParserCollection, nil
}

func newTraceParserCollection() (*genericParserCollection, error) {
	telemetrySettings := component.TelemetrySettings{
		Logger: zap.New(zapcore.NewNopCore()),
	}

	parserCollectionOpts := []genericParserCollectionOption{
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

	parserCollectionOpts = append(parserCollectionOpts, withCommonContextsParsers()...)

	traceParserCollection, err := newGenericParserCollection(telemetrySettings, parserCollectionOpts...)

	if err != nil {
		return nil, fmt.Errorf("failed to create trace parser collection: %w", err)
	}

	return traceParserCollection, nil
}

func withCommonContextsParsers() []genericParserCollectionOption {
	return []genericParserCollectionOption{
		withResourceParser(
			ottlfuncs.StandardFuncs[ottlresource.TransformContext](),
			ottl.WithStatementConverter(convertResourceStatements),
			ottl.WithConditionConverter(convertResourceConditions),
		),
		withScopeParser(
			ottlfuncs.StandardFuncs[ottlscope.TransformContext](),
			ottl.WithStatementConverter(convertScopeStatements),
			ottl.WithConditionConverter(convertScopeConditions),
		),
	}
}
