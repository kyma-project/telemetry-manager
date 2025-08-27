package transformspec

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"go.opentelemetry.io/collector/component"
)

type genericParserCollection ottl.ParserCollection[any]

type genericParserCollectionOption ottl.ParserCollectionOption[any]

func withLogParser(functions map[string]ottl.Factory[ottllog.TransformContext]) genericParserCollectionOption {
	return func(pc *ottl.ParserCollection[any]) error {
		logParser, err := ottllog.NewParser(functions, pc.Settings, ottllog.EnablePathContextNames())
		if err != nil {
			return err
		}

		return ottl.WithParserCollectionContext(
			ottllog.ContextName,
			&logParser,
			ottl.WithStatementConverter(convertLogStatements),
			ottl.WithConditionConverter(convertLogConditions),
		)(pc)
	}
}

func withDataPointParser(functions map[string]ottl.Factory[ottldatapoint.TransformContext]) genericParserCollectionOption {
	return func(pc *ottl.ParserCollection[any]) error {
		datapointParser, err := ottldatapoint.NewParser(functions, pc.Settings, ottldatapoint.EnablePathContextNames())
		if err != nil {
			return err
		}

		return ottl.WithParserCollectionContext(
			ottldatapoint.ContextName,
			&datapointParser,
			ottl.WithStatementConverter(convertDataPointStatements),
			ottl.WithConditionConverter(convertDataPointConditions),
		)(pc)
	}
}

func withMetricParser(functions map[string]ottl.Factory[ottlmetric.TransformContext]) genericParserCollectionOption {
	return func(pc *ottl.ParserCollection[any]) error {
		metricParser, err := ottlmetric.NewParser(functions, pc.Settings, ottlmetric.EnablePathContextNames())
		if err != nil {
			return err
		}

		return ottl.WithParserCollectionContext(
			ottlmetric.ContextName,
			&metricParser,
			ottl.WithStatementConverter(convertMetricStatements),
			ottl.WithConditionConverter(convertMetricConditions),
		)(pc)
	}
}

func newGenericParserCollection(settings component.TelemetrySettings, options ...genericParserCollectionOption) (*genericParserCollection, error) {
	pcOptions := []ottl.ParserCollectionOption[any]{
		withCommonContextParsers(),
		ottl.EnableParserCollectionModifiedPathsLogging[any](true),
	}

	for _, option := range options {
		pcOptions = append(pcOptions, ottl.ParserCollectionOption[any](option))
	}

	pc, err := ottl.NewParserCollection(settings, pcOptions...)
	if err != nil {
		return nil, err
	}

	gpc := genericParserCollection(*pc)

	return &gpc, nil
}

func (gpc *genericParserCollection) parseStatementsAndConditions(statements []string, conditions []string) error {
	pc := ottl.ParserCollection[any](*gpc)

	if _, err := pc.ParseStatements(ottl.NewStatementsGetter(statements), ottl.WithContextInferenceConditions(conditions)); err != nil {
		return err
	}

	if len(conditions) == 0 {
		return nil
	}

	if _, err := pc.ParseConditions(ottl.NewConditionsGetter(conditions)); err != nil {
		return err
	}

	return nil
}

func withCommonContextParsers() ottl.ParserCollectionOption[any] {
	return func(pc *ottl.ParserCollection[any]) error {
		rp, err := ottlresource.NewParser(ottlfuncs.StandardFuncs[ottlresource.TransformContext](), pc.Settings, ottlresource.EnablePathContextNames())
		if err != nil {
			return err
		}

		sp, err := ottlscope.NewParser(ottlfuncs.StandardFuncs[ottlscope.TransformContext](), pc.Settings, ottlscope.EnablePathContextNames())
		if err != nil {
			return err
		}

		err = ottl.WithParserCollectionContext(
			ottlresource.ContextName,
			&rp,
			ottl.WithStatementConverter(convertResourceStatements),
			ottl.WithConditionConverter(convertResourceConditions),
		)(pc)
		if err != nil {
			return err
		}

		err = ottl.WithParserCollectionContext(
			ottlscope.ContextName,
			&sp,
			ottl.WithStatementConverter(convertScopeStatements),
			ottl.WithConditionConverter(convertScopeConditions),
		)(pc)
		if err != nil {
			return err
		}

		return nil
	}
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
