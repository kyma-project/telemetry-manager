package ottl

import (
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

type FilterSpecValidator struct {
	parserCollection *genericParserCollection
}

func NewFilterSpecValidator(signalType SignalType) (*FilterSpecValidator, error) {
	opts, err := newFilterParserCollectionOpts(signalType)
	if err != nil {
		return nil, err
	}

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

func newFilterParserCollectionOpts(signalType SignalType) ([]genericParserCollectionOption, error) {
	var opts []genericParserCollectionOption

	switch signalType {
	case SignalTypeTrace:
		opts = append(opts,
			withSpanParser(
				ottlfuncs.StandardConverters[ottlspan.TransformContext](),
				ottl.WithConditionConverter(convertSpanConditions),
			),
		)
	default:
		// return nil, fmt.Errorf("unsupported signal type: %s", signalType)
	}

	// Always include common context parsers, no matter the signal type
	opts = append(opts,
		withResourceParser(
			// Include all standard OTTL converters (NO editors) for resource context
			ottlfuncs.StandardConverters[ottlresource.TransformContext](),
			ottl.WithConditionConverter(convertResourceConditions),
		),
		withScopeParser(
			// Include all standard OTTL converters (NO editors) for scope context
			ottlfuncs.StandardConverters[ottlscope.TransformContext](),
			ottl.WithConditionConverter(convertScopeConditions),
		),
	)

	return opts, nil
}
