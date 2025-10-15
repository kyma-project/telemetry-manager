package ottl

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type genericParserCollection ottl.ParserCollection[any]

type genericParserCollectionOption ottl.ParserCollectionOption[any]

func withLogParser(functions map[string]ottl.Factory[ottllog.TransformContext], opts ...ottl.ParserCollectionContextOption[ottllog.TransformContext, any]) genericParserCollectionOption {
	return func(pc *ottl.ParserCollection[any]) error {
		logParser, err := ottllog.NewParser(functions, pc.Settings, ottllog.EnablePathContextNames())
		if err != nil {
			return err
		}

		return ottl.WithParserCollectionContext(
			ottllog.ContextName,
			&logParser,
			opts...,
		)(pc)
	}
}

func withDataPointParser(functions map[string]ottl.Factory[ottldatapoint.TransformContext], opts ...ottl.ParserCollectionContextOption[ottldatapoint.TransformContext, any]) genericParserCollectionOption {
	return func(pc *ottl.ParserCollection[any]) error {
		datapointParser, err := ottldatapoint.NewParser(functions, pc.Settings, ottldatapoint.EnablePathContextNames())
		if err != nil {
			return err
		}

		return ottl.WithParserCollectionContext(
			ottldatapoint.ContextName,
			&datapointParser,
			opts...,
		)(pc)
	}
}

func withMetricParser(functions map[string]ottl.Factory[ottlmetric.TransformContext], opts ...ottl.ParserCollectionContextOption[ottlmetric.TransformContext, any]) genericParserCollectionOption {
	return func(pc *ottl.ParserCollection[any]) error {
		metricParser, err := ottlmetric.NewParser(functions, pc.Settings, ottlmetric.EnablePathContextNames())
		if err != nil {
			return err
		}

		return ottl.WithParserCollectionContext(
			ottlmetric.ContextName,
			&metricParser,
			opts...,
		)(pc)
	}
}

func withSpanEventParser(functions map[string]ottl.Factory[ottlspanevent.TransformContext], opts ...ottl.ParserCollectionContextOption[ottlspanevent.TransformContext, any]) genericParserCollectionOption {
	return func(pc *ottl.ParserCollection[any]) error {
		spanEventParser, err := ottlspanevent.NewParser(functions, pc.Settings, ottlspanevent.EnablePathContextNames())
		if err != nil {
			return err
		}

		return ottl.WithParserCollectionContext(
			ottlspanevent.ContextName,
			&spanEventParser,
			opts...,
		)(pc)
	}
}

func withSpanParser(functions map[string]ottl.Factory[ottlspan.TransformContext], opts ...ottl.ParserCollectionContextOption[ottlspan.TransformContext, any]) genericParserCollectionOption {
	return func(pc *ottl.ParserCollection[any]) error {
		spanParser, err := ottlspan.NewParser(functions, pc.Settings, ottlspan.EnablePathContextNames())
		if err != nil {
			return err
		}

		return ottl.WithParserCollectionContext(
			ottlspan.ContextName,
			&spanParser,
			opts...,
		)(pc)
	}
}

func withResourceParser(functions map[string]ottl.Factory[ottlresource.TransformContext], opts ...ottl.ParserCollectionContextOption[ottlresource.TransformContext, any]) genericParserCollectionOption {
	return func(pc *ottl.ParserCollection[any]) error {
		resourceParser, err := ottlresource.NewParser(functions, pc.Settings, ottlresource.EnablePathContextNames())
		if err != nil {
			return err
		}

		return ottl.WithParserCollectionContext(
			ottlresource.ContextName,
			&resourceParser,
			opts...,
		)(pc)
	}
}

func withScopeParser(functions map[string]ottl.Factory[ottlscope.TransformContext], opts ...ottl.ParserCollectionContextOption[ottlscope.TransformContext, any]) genericParserCollectionOption {
	return func(pc *ottl.ParserCollection[any]) error {
		scopeParser, err := ottlscope.NewParser(functions, pc.Settings, ottlscope.EnablePathContextNames())
		if err != nil {
			return err
		}

		return ottl.WithParserCollectionContext(
			ottlscope.ContextName,
			&scopeParser,
			opts...,
		)(pc)
	}
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

func newGenericParserCollection(options ...genericParserCollectionOption) (*genericParserCollection, error) {
	parserCollectionOptions := []ottl.ParserCollectionOption[any]{
		ottl.EnableParserCollectionModifiedPathsLogging[any](true),
	}

	for _, option := range options {
		parserCollectionOptions = append(parserCollectionOptions, ottl.ParserCollectionOption[any](option))
	}

	telemetrySettings := component.TelemetrySettings{
		Logger: zap.New(zapcore.NewNopCore()),
	}

	parserCollection, err := ottl.NewParserCollection(telemetrySettings, parserCollectionOptions...)
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

func convertResourceStatements(_ *ottl.ParserCollection[any], _ ottl.StatementsGetter, _ []*ottl.Statement[ottlresource.TransformContext]) (any, error) {
	return struct{}{}, nil
}

func convertResourceConditions(_ *ottl.ParserCollection[any], _ ottl.ConditionsGetter, _ []*ottl.Condition[ottlresource.TransformContext]) (any, error) {
	return struct{}{}, nil
}

func convertScopeStatements(_ *ottl.ParserCollection[any], _ ottl.StatementsGetter, _ []*ottl.Statement[ottlscope.TransformContext]) (any, error) {
	return struct{}{}, nil
}

func convertScopeConditions(_ *ottl.ParserCollection[any], _ ottl.ConditionsGetter, _ []*ottl.Condition[ottlscope.TransformContext]) (any, error) {
	return struct{}{}, nil
}

func convertLogStatements(_ *ottl.ParserCollection[any], _ ottl.StatementsGetter, _ []*ottl.Statement[ottllog.TransformContext]) (any, error) {
	return struct{}{}, nil
}

func convertLogConditions(_ *ottl.ParserCollection[any], _ ottl.ConditionsGetter, _ []*ottl.Condition[ottllog.TransformContext]) (any, error) {
	return struct{}{}, nil
}

func convertDataPointStatements(_ *ottl.ParserCollection[any], _ ottl.StatementsGetter, _ []*ottl.Statement[ottldatapoint.TransformContext]) (any, error) {
	return struct{}{}, nil
}

func convertDataPointConditions(_ *ottl.ParserCollection[any], _ ottl.ConditionsGetter, _ []*ottl.Condition[ottldatapoint.TransformContext]) (any, error) {
	return struct{}{}, nil
}

func convertMetricStatements(_ *ottl.ParserCollection[any], _ ottl.StatementsGetter, _ []*ottl.Statement[ottlmetric.TransformContext]) (any, error) {
	return struct{}{}, nil
}

func convertMetricConditions(_ *ottl.ParserCollection[any], _ ottl.ConditionsGetter, _ []*ottl.Condition[ottlmetric.TransformContext]) (any, error) {
	return struct{}{}, nil
}

func convertSpanEventStatements(_ *ottl.ParserCollection[any], _ ottl.StatementsGetter, _ []*ottl.Statement[ottlspanevent.TransformContext]) (any, error) {
	return struct{}{}, nil
}

func convertSpanEventConditions(_ *ottl.ParserCollection[any], _ ottl.ConditionsGetter, _ []*ottl.Condition[ottlspanevent.TransformContext]) (any, error) {
	return struct{}{}, nil
}

func convertSpanStatements(_ *ottl.ParserCollection[any], _ ottl.StatementsGetter, _ []*ottl.Statement[ottlspan.TransformContext]) (any, error) {
	return struct{}{}, nil
}

func convertSpanConditions(_ *ottl.ParserCollection[any], _ ottl.ConditionsGetter, _ []*ottl.Condition[ottlspan.TransformContext]) (any, error) {
	return struct{}{}, nil
}
