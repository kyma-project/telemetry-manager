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
	// The statementsParserCollection is intentionally not exported, because it should be initialized internally only by the constructor
	statementsParserCollection *genericParserCollection
	// The conditionsParserCollection is intentionally not exported, because it should be initialized internally only by the constructor
	conditionsParserCollection *genericParserCollection
}

func New(signalType SignalType) (*Validator, error) {
	var err error

	validator := &Validator{}

	switch signalType {
	case SignalTypeLog:
		err = validator.setLogParserCollections()
	case SignalTypeMetric:
		err = validator.setMetricParserCollections()
	case SignalTypeTrace:
		err = validator.setTraceParserCollections()
	default:
		return nil, fmt.Errorf("unexpected signal type: %s", signalType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create TransformSpec validator: %w", err)
	}

	return validator, nil
}

func (v *Validator) setLogParserCollections() error {
	telemetrySettings := component.TelemetrySettings{
		Logger: zap.New(zapcore.NewNopCore()),
	}

	// Set log statements parser collection
	logStatementsFunctions := ottl.CreateFactoryMap(transformprocessor.DefaultLogFunctions()...)

	logStatementsParserCollection, err := newGenericParserCollection(
		telemetrySettings,
		withResourceParser(ottlfuncs.StandardFuncs[ottlresource.TransformContext](), ottl.WithStatementConverter(convertResourceStatements)),
		withScopeParser(ottlfuncs.StandardFuncs[ottlscope.TransformContext](), ottl.WithStatementConverter(convertScopeStatements)),
		withLogParser(logStatementsFunctions, ottl.WithStatementConverter(convertLogStatements)),
	)

	if err != nil {
		return fmt.Errorf("failed to create log statements parser collection: %w", err)
	}

	v.statementsParserCollection = logStatementsParserCollection

	// Set log conditions parser collection
	logConditionsParserCollection, err := newGenericParserCollection(
		telemetrySettings,
		withResourceParser(resourceConverterFuncs(), ottl.WithConditionConverter(convertResourceConditions)),
		withScopeParser(scopeConverterFuncs(), ottl.WithConditionConverter(convertScopeConditions)),
		withLogParser(logConverterFuncs(), ottl.WithConditionConverter(convertLogConditions)),
	)

	if err != nil {
		return fmt.Errorf("failed to create log conditions parser collection: %w", err)
	}

	v.conditionsParserCollection = logConditionsParserCollection

	return nil
}

func (v *Validator) setMetricParserCollections() error {
	telemetrySettings := component.TelemetrySettings{
		Logger: zap.New(zapcore.NewNopCore()),
	}

	// Set metric statements parser collection
	metricStatementsFunctions := ottl.CreateFactoryMap(transformprocessor.DefaultMetricFunctions()...)
	dataPointStatementsFunctions := ottl.CreateFactoryMap(transformprocessor.DefaultDataPointFunctions()...)

	metricStatementsParserCollection, err := newGenericParserCollection(
		telemetrySettings,
		withResourceParser(ottlfuncs.StandardFuncs[ottlresource.TransformContext](), ottl.WithStatementConverter(convertResourceStatements)),
		withScopeParser(ottlfuncs.StandardFuncs[ottlscope.TransformContext](), ottl.WithStatementConverter(convertScopeStatements)),
		withMetricParser(metricStatementsFunctions, ottl.WithStatementConverter(convertMetricStatements)),
		withDataPointParser(dataPointStatementsFunctions, ottl.WithStatementConverter(convertDataPointStatements)),
	)

	if err != nil {
		return fmt.Errorf("failed to create metric statements parser collection: %w", err)
	}

	v.statementsParserCollection = metricStatementsParserCollection

	// Set metric conditions parser collection
	metricConditionsParserCollection, err := newGenericParserCollection(
		telemetrySettings,
		withResourceParser(resourceConverterFuncs(), ottl.WithConditionConverter(convertResourceConditions)),
		withScopeParser(scopeConverterFuncs(), ottl.WithConditionConverter(convertScopeConditions)),
		withMetricParser(metricConverterFuncs(), ottl.WithConditionConverter(convertMetricConditions)),
		withDataPointParser(dataPointConverterFuncs(), ottl.WithConditionConverter(convertDataPointConditions)),
	)

	if err != nil {
		return fmt.Errorf("failed to create metric conditions parser collection: %w", err)
	}

	v.conditionsParserCollection = metricConditionsParserCollection

	return nil
}

func (v *Validator) setTraceParserCollections() error {
	telemetrySettings := component.TelemetrySettings{
		Logger: zap.New(zapcore.NewNopCore()),
	}

	// Set trace statements parser collection
	spanStatementsFunctionsMap := ottl.CreateFactoryMap(transformprocessor.DefaultSpanFunctions()...)
	spanEventStatementsFunctionsMap := ottl.CreateFactoryMap(transformprocessor.DefaultSpanEventFunctions()...)

	traceStatementsParserCollection, err := newGenericParserCollection(
		telemetrySettings,
		withResourceParser(ottlfuncs.StandardFuncs[ottlresource.TransformContext](), ottl.WithStatementConverter(convertResourceStatements)),
		withScopeParser(ottlfuncs.StandardFuncs[ottlscope.TransformContext](), ottl.WithStatementConverter(convertScopeStatements)),
		withSpanParser(spanStatementsFunctionsMap, ottl.WithStatementConverter(convertSpanStatements)),
		withSpanEventParser(spanEventStatementsFunctionsMap, ottl.WithStatementConverter(convertSpanEventStatements)),
	)

	if err != nil {
		return fmt.Errorf("failed to create trace statements parser collection: %w", err)
	}

	v.statementsParserCollection = traceStatementsParserCollection

	// Set trace conditions parser collection
	traceConditionsParserCollection, err := newGenericParserCollection(
		telemetrySettings,
		withResourceParser(resourceConverterFuncs(), ottl.WithConditionConverter(convertResourceConditions)),
		withScopeParser(scopeConverterFuncs(), ottl.WithConditionConverter(convertScopeConditions)),
		withSpanParser(spanConverterFuncs(), ottl.WithConditionConverter(convertSpanConditions)),
		withSpanEventParser(spanEventConverterFuncs(), ottl.WithConditionConverter(convertSpanEventConditions)),
	)

	if err != nil {
		return fmt.Errorf("failed to create trace conditions parser collection: %w", err)
	}

	v.conditionsParserCollection = traceConditionsParserCollection

	return nil
}

func (v *Validator) Validate(transforms []telemetryv1alpha1.TransformSpec) error {
	for _, ts := range transforms {
		if err := v.statementsParserCollection.parseStatementsWithConditions(ts.Statements, ts.Conditions); err != nil {
			return &InvalidTransformSpecError{Err: fmt.Errorf("invalid TransformSpec: %w", err)}
		}

		if err := v.conditionsParserCollection.parseConditions(ts.Conditions); err != nil {
			return &InvalidTransformSpecError{Err: fmt.Errorf("invalid TransformSpec: %w", err)}
		}
	}

	return nil
}
