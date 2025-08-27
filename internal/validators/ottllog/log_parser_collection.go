package ottllog

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

type genericParserCollection ottl.ParserCollection[any]

type genericParserCollectionOption ottl.ParserCollectionOption[any]

func withLogParser(functions map[string]ottl.Factory[ottllog.TransformContext]) genericParserCollectionOption {
	return func(pc *ottl.ParserCollection[any]) error {
		logParser, err := ottllog.NewParser(functions, pc.Settings, ottllog.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottllog.ContextName, &logParser, ottl.WithStatementConverter(convertLogStatements))(pc)
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

func (gpc *genericParserCollection) parseStatementsWithConditions(statements []string, conditions []string) (any, error) {
	pc := ottl.ParserCollection[any](*gpc)
	return pc.ParseStatements(ottl.NewStatementsGetter(statements), ottl.WithContextInferenceConditions(conditions))
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

		err = ottl.WithParserCollectionContext(ottlresource.ContextName, &rp, ottl.WithStatementConverter(convertResourceStatements))(pc)
		if err != nil {
			return err
		}

		err = ottl.WithParserCollectionContext(ottlscope.ContextName, &sp, ottl.WithStatementConverter(convertScopeStatements))(pc)
		if err != nil {
			return err
		}

		return nil
	}
}

func convertLogStatements(_ *ottl.ParserCollection[any], _ ottl.StatementsGetter, _ []*ottl.Statement[ottllog.TransformContext]) (any, error) {
	return nil, nil
}

func convertResourceStatements(_ *ottl.ParserCollection[any], _ ottl.StatementsGetter, _ []*ottl.Statement[ottlresource.TransformContext]) (any, error) {
	return nil, nil
}

func convertScopeStatements(_ *ottl.ParserCollection[any], _ ottl.StatementsGetter, _ []*ottl.Statement[ottlscope.TransformContext]) (any, error) {
	return nil, nil
}
