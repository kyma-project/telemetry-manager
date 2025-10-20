package ottl

import (
	"errors"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type InvalidOTTLSpecError struct {
	Err error
}

func (e *InvalidOTTLSpecError) Error() string {
	return e.Err.Error()
}

func IsInvalidOTTLSpecError(err error) bool {
	var errInvalidTransformSpec *InvalidOTTLSpecError
	return errors.As(err, &errInvalidTransformSpec)
}

type SignalType string

const (
	SignalTypeLog    SignalType = "log"
	SignalTypeMetric SignalType = "metric"
	SignalTypeTrace  SignalType = "trace"
)

type TransformSpecValidator struct {
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
			return &InvalidOTTLSpecError{Err: fmt.Errorf("%s: %w", errorMessage, err)}
		}

		if err := v.parserCollection.parseConditions(ts.Conditions); err != nil {
			return &InvalidOTTLSpecError{Err: fmt.Errorf("%s: %w", errorMessage, err)}
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
				ottl.WithStatementConverter(nopStatementConverter[ottllog.TransformContext]),
				ottl.WithConditionConverter(nopConditionConverter[ottllog.TransformContext]),
			),
		}
	case SignalTypeMetric:
		opts = []genericParserCollectionOption{
			withMetricParser(
				ottl.CreateFactoryMap(transformprocessor.DefaultMetricFunctions()...),
				ottl.WithStatementConverter(nopStatementConverter[ottlmetric.TransformContext]),
				ottl.WithConditionConverter(nopConditionConverter[ottlmetric.TransformContext]),
			),
			withDataPointParser(
				ottl.CreateFactoryMap(transformprocessor.DefaultDataPointFunctions()...),
				ottl.WithStatementConverter(nopStatementConverter[ottldatapoint.TransformContext]),
				ottl.WithConditionConverter(nopConditionConverter[ottldatapoint.TransformContext]),
			),
		}
	case SignalTypeTrace:
		opts = []genericParserCollectionOption{
			withSpanParser(
				ottl.CreateFactoryMap(transformprocessor.DefaultSpanFunctions()...),
				ottl.WithStatementConverter(nopStatementConverter[ottlspan.TransformContext]),
				ottl.WithConditionConverter(nopConditionConverter[ottlspan.TransformContext]),
			),
			withSpanEventParser(
				ottl.CreateFactoryMap(transformprocessor.DefaultSpanEventFunctions()...),
				ottl.WithStatementConverter(nopStatementConverter[ottlspanevent.TransformContext]),
				ottl.WithConditionConverter(nopConditionConverter[ottlspanevent.TransformContext]),
			),
		}
	default:
		return nil, fmt.Errorf("unexpected signal type: %s", signalType)
	}

	// Always include common context parsers, no matter the signal type
	opts = append(opts,
		withResourceParser(
			// Include all standard OTTL functions (editors and converters) for resource context
			ottlfuncs.StandardFuncs[ottlresource.TransformContext](),
			ottl.WithStatementConverter(nopStatementConverter[ottlresource.TransformContext]),
			ottl.WithConditionConverter(nopConditionConverter[ottlresource.TransformContext]),
		),
		withScopeParser(
			// Include all standard OTTL functions (editors and converters) for scope context
			ottlfuncs.StandardFuncs[ottlscope.TransformContext](),
			ottl.WithStatementConverter(nopStatementConverter[ottlscope.TransformContext]),
			ottl.WithConditionConverter(nopConditionConverter[ottlscope.TransformContext]),
		),
	)

	return opts, nil
}
