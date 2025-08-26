// Copied over from "github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl/filter.go"

package ottllog

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
)

// NewBoolExprForResourceWithOptions is like NewBoolExprForResource, but with additional options.
func NewBoolExprForResourceWithOptions(conditions []string, functions map[string]ottl.Factory[ottlresource.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings, parserOptions []ottl.Option[ottlresource.TransformContext]) (*ottl.ConditionSequence[ottlresource.TransformContext], error) {
	parser, err := ottlresource.NewParser(functions, set, parserOptions...)
	if err != nil {
		return nil, err
	}
	statements, err := parser.ParseConditions(conditions)
	if err != nil {
		return nil, err
	}
	c := ottlresource.NewConditionSequence(statements, set, ottlresource.WithConditionSequenceErrorMode(errorMode))
	return &c, nil
}

// NewBoolExprForScopeWithOptions is like NewBoolExprForScope, but with additional options.
func NewBoolExprForScopeWithOptions(conditions []string, functions map[string]ottl.Factory[ottlscope.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings, parserOptions []ottl.Option[ottlscope.TransformContext]) (*ottl.ConditionSequence[ottlscope.TransformContext], error) {
	parser, err := ottlscope.NewParser(functions, set, parserOptions...)
	if err != nil {
		return nil, err
	}
	statements, err := parser.ParseConditions(conditions)
	if err != nil {
		return nil, err
	}
	c := ottlscope.NewConditionSequence(statements, set, ottlscope.WithConditionSequenceErrorMode(errorMode))
	return &c, nil
}

// NewBoolExprForLogWithOptions is like NewBoolExprForLog, but with additional options.
func NewBoolExprForLogWithOptions(conditions []string, functions map[string]ottl.Factory[ottllog.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings, parserOptions []ottl.Option[ottllog.TransformContext]) (*ottl.ConditionSequence[ottllog.TransformContext], error) {
	parser, err := ottllog.NewParser(functions, set, parserOptions...)
	if err != nil {
		return nil, err
	}
	statements, err := parser.ParseConditions(conditions)
	if err != nil {
		return nil, err
	}
	c := ottllog.NewConditionSequence(statements, set, ottllog.WithConditionSequenceErrorMode(errorMode))
	return &c, nil
}
