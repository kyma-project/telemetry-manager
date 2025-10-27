package ottl

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type FilterSpecValidator struct {
	parserCollection *genericParserCollection
}

func NewFilterSpecValidator(signalType SignalType) (*FilterSpecValidator, error) {
	if err := signalType.Validate(); err != nil {
		return nil, err
	}

	opts := newFilterParserCollectionOpts(signalType)

	parserCollection, err := newGenericParserCollection(opts...)
	if err != nil {
		return nil, err
	}

	return &FilterSpecValidator{parserCollection: parserCollection}, nil
}

func (v *FilterSpecValidator) Validate(filters []telemetryv1alpha1.FilterSpec) error {
	const errorMessage = "invalid FilterSpec"

	for _, fs := range filters {
		if err := v.parserCollection.parseConditions(fs.Conditions); err != nil {
			return &InvalidOTTLSpecError{Err: fmt.Errorf("%s: %w", errorMessage, err)}
		}
	}

	return nil
}

func newFilterParserCollectionOpts(signalType SignalType) []genericParserCollectionOption {
	var opts []genericParserCollectionOption

	switch signalType {
	case SignalTypeTrace:
		opts = append(opts,
			// Since context inference is not available in the filter processor yet,
			// we set the context to span as the minimum required context.
			// Span event context is not supported.
			withSpanParser(
				ottl.CreateFactoryMap(filterprocessor.DefaultSpanFunctions()...),
				ottl.WithConditionConverter(nopConditionConverter[ottlspan.TransformContext]),
			),
		)
	case SignalTypeLog:
		opts = append(opts,
			withLogParser(
				ottl.CreateFactoryMap(filterprocessor.DefaultLogFunctions()...),
				ottl.WithConditionConverter(nopConditionConverter[ottllog.TransformContext]),
			),
		)
	case SignalTypeMetric:
		opts = append(opts,
			// Since context inference is not available in the filter processor yet,
			// we set the context to datapoint as the minimum required context.
			// That is why metric-context-only functions (like HasAttrKeyOnDatapoint or HasAttrOnDatapoint) are not supported here
			// and only standard converters are included.
			withMetricParser(
				ottlfuncs.StandardConverters[ottlmetric.TransformContext](),
				ottl.WithConditionConverter(nopConditionConverter[ottlmetric.TransformContext]),
			),
			withDataPointParser(
				ottl.CreateFactoryMap(filterprocessor.DefaultDataPointFunctions()...),
				ottl.WithConditionConverter(nopConditionConverter[ottldatapoint.TransformContext]),
			),
		)
	}

	// Always include common context parsers, no matter the signal type
	opts = append(opts,
		withResourceParser(
			// Include all standard OTTL converters (NO editors) for resource context
			ottlfuncs.StandardConverters[ottlresource.TransformContext](),
			ottl.WithConditionConverter(nopConditionConverter[ottlresource.TransformContext]),
		),
		withScopeParser(
			// Include all standard OTTL converters (NO editors) for scope context
			ottlfuncs.StandardConverters[ottlscope.TransformContext](),
			ottl.WithConditionConverter(nopConditionConverter[ottlscope.TransformContext]),
		),
	)

	return opts
}
