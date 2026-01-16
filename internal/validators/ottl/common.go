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
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

func (s SignalType) Validate() error {
	switch s {
	case SignalTypeLog, SignalTypeMetric, SignalTypeTrace:
		return nil
	default:
		return fmt.Errorf("invalid SignalType: %s", s)
	}
}

// NOTE: The following statements apply to OTel Collector Contrib v0.136.0 and may change in future versions.
// The transform processor uses ottl.ParserCollection[T], which supports both hard-coded context and context inference.
// The filter processor does not yet use ottl.ParserCollection[T] because it lacks context inference support. Instead, it uses ottl.ConditionSequence[T].
// However, we use ottl.ParserCollection[T] for both validators here to provide a consistent experience for our users.
// Essentially, the real filter processor supports missing context in paths, while our validator does not. In the future the filter processor will supports context inference and use ParserCollection[T].
// You can see this in their draft PR: https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/39688.

// genericParserCollection is a wrapper around ottl.ParserCollection[any] that simplifies the API
// to be used in the TransformSpec and FilterSpec validators.
type genericParserCollection ottl.ParserCollection[any]

type genericParserCollectionOption ottl.ParserCollectionOption[any]

func withLogParser(functions map[string]ottl.Factory[*ottllog.TransformContext], opts ...ottl.ParserCollectionContextOption[*ottllog.TransformContext, any]) genericParserCollectionOption {
	return func(pc *ottl.ParserCollection[any]) error {
		logParser, err := ottllog.NewParser(functions, pc.Settings, ottllog.EnablePathContextNames())
		if err != nil {
			return err
		}

		return ottl.WithParserCollectionContext(ottllog.ContextName, &logParser, opts...)(pc)
	}
}

func withDataPointParser(functions map[string]ottl.Factory[*ottldatapoint.TransformContext], opts ...ottl.ParserCollectionContextOption[*ottldatapoint.TransformContext, any]) genericParserCollectionOption {
	return func(pc *ottl.ParserCollection[any]) error {
		datapointParser, err := ottldatapoint.NewParser(functions, pc.Settings, ottldatapoint.EnablePathContextNames())
		if err != nil {
			return err
		}

		return ottl.WithParserCollectionContext(ottldatapoint.ContextName, &datapointParser, opts...)(pc)
	}
}

func withMetricParser(functions map[string]ottl.Factory[*ottlmetric.TransformContext], opts ...ottl.ParserCollectionContextOption[*ottlmetric.TransformContext, any]) genericParserCollectionOption {
	return func(pc *ottl.ParserCollection[any]) error {
		metricParser, err := ottlmetric.NewParser(functions, pc.Settings, ottlmetric.EnablePathContextNames())
		if err != nil {
			return err
		}

		return ottl.WithParserCollectionContext(ottlmetric.ContextName, &metricParser, opts...)(pc)
	}
}

func withSpanEventParser(functions map[string]ottl.Factory[*ottlspanevent.TransformContext], opts ...ottl.ParserCollectionContextOption[*ottlspanevent.TransformContext, any]) genericParserCollectionOption {
	return func(pc *ottl.ParserCollection[any]) error {
		spanEventParser, err := ottlspanevent.NewParser(functions, pc.Settings, ottlspanevent.EnablePathContextNames())
		if err != nil {
			return err
		}

		return ottl.WithParserCollectionContext(ottlspanevent.ContextName, &spanEventParser, opts...)(pc)
	}
}

func withSpanParser(functions map[string]ottl.Factory[*ottlspan.TransformContext], opts ...ottl.ParserCollectionContextOption[*ottlspan.TransformContext, any]) genericParserCollectionOption {
	return func(pc *ottl.ParserCollection[any]) error {
		spanParser, err := ottlspan.NewParser(functions, pc.Settings, ottlspan.EnablePathContextNames())
		if err != nil {
			return err
		}

		return ottl.WithParserCollectionContext(ottlspan.ContextName, &spanParser, opts...)(pc)
	}
}

func withResourceParser(functions map[string]ottl.Factory[*ottlresource.TransformContext], opts ...ottl.ParserCollectionContextOption[*ottlresource.TransformContext, any]) genericParserCollectionOption {
	return func(pc *ottl.ParserCollection[any]) error {
		resourceParser, err := ottlresource.NewParser(functions, pc.Settings, ottlresource.EnablePathContextNames())
		if err != nil {
			return err
		}

		return ottl.WithParserCollectionContext(ottlresource.ContextName, &resourceParser, opts...)(pc)
	}
}

func withScopeParser(functions map[string]ottl.Factory[*ottlscope.TransformContext], opts ...ottl.ParserCollectionContextOption[*ottlscope.TransformContext, any]) genericParserCollectionOption {
	return func(pc *ottl.ParserCollection[any]) error {
		scopeParser, err := ottlscope.NewParser(functions, pc.Settings, ottlscope.EnablePathContextNames())
		if err != nil {
			return err
		}

		return ottl.WithParserCollectionContext(ottlscope.ContextName, &scopeParser, opts...)(pc)
	}
}

func newGenericParserCollection(opts ...genericParserCollectionOption) (*genericParserCollection, error) {
	ottlOpts := []ottl.ParserCollectionOption[any]{
		ottl.EnableParserCollectionModifiedPathsLogging[any](true),
	}

	for _, option := range opts {
		ottlOpts = append(ottlOpts, ottl.ParserCollectionOption[any](option))
	}

	telemetrySettings := component.TelemetrySettings{
		Logger: zap.New(zapcore.NewNopCore()),
	}

	parserCollection, err := ottl.NewParserCollection(telemetrySettings, ottlOpts...)
	if err != nil {
		return nil, err
	}

	gpc := genericParserCollection(*parserCollection)

	return &gpc, nil
}

func (gpc *genericParserCollection) parseStatementsWithConditions(statements []string, conditions []string) error {
	parserCollection := ottl.ParserCollection[any](*gpc)

	if _, err := parserCollection.ParseStatements(ottl.NewStatementsGetter(statements), ottl.WithContextInferenceConditions(conditions)); err != nil {
		return err
	}

	return nil
}

func (gpc *genericParserCollection) parseConditions(conditions []string) error {
	if len(conditions) == 0 {
		return nil
	}

	parserCollection := ottl.ParserCollection[any](*gpc)

	if _, err := parserCollection.ParseConditions(ottl.NewConditionsGetter(conditions)); err != nil {
		return err
	}

	return nil
}

// Since we do not need to evluate OTTL expressions in the validators, these converters are no-ops that just return the parsed conditions/statements.

func nopConditionConverter[T any](_ *ottl.ParserCollection[any], _ ottl.ConditionsGetter, parsedConditions []*ottl.Condition[T]) (any, error) {
	return parsedConditions, nil
}

func nopStatementConverter[T any](_ *ottl.ParserCollection[any], _ ottl.StatementsGetter, parsedStatements []*ottl.Statement[T]) (any, error) {
	return parsedStatements, nil
}
