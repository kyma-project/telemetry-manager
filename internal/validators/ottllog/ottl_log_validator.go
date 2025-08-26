package ottllog

import (
	"context"
	"errors"
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type InvalidOTTLExpressionError struct {
	Err error
}

func (e *InvalidOTTLExpressionError) Error() string {
	return e.Err.Error()
}

func IsInvalidOTTLExpressionError(err error) bool {
	var errInvalidOTTLExpression *InvalidOTTLExpressionError
	return errors.As(err, &errInvalidOTTLExpression)
}

type Validator struct {
}

func (v *Validator) Validate(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	// for _, transformSpec := range pipeline.Spec.Transforms {
	// 	if err := v.validateTransformSpec(ctx, transformSpec.Conditions, transformSpec.Statements); err != nil {
	// 		return err
	// 	}
	// }

	if err := v.validateTransformSpec(ctx, pipeline.Spec.Transforms); err != nil {
		return err
	}

	return nil
}

// func (v *Validator) validateTransformSpec(ctx context.Context, conditions []string, statements []string) error {
// 	telemetrySettings := component.TelemetrySettings{
// 		Logger: zap.New(zapcore.NewNopCore()),
// 	}

// 	functionsMap := ottl.CreateFactoryMap(transformprocessor.DefaultLogFunctions()...)

// 	parser, err := ottllog.NewParser(functionsMap, telemetrySettings, ottllog.EnablePathContextNames())
// 	if err != nil {
// 		logf.FromContext(ctx).Error(err, "Failed to create OTTL parser")
// 		return nil
// 	}

// 	if _, err := parser.ParseConditions(conditions); err != nil {
// 		return &InvalidOTTLExpressionError{Err: fmt.Errorf("invalid condition(s) in Transform spec: %w", err)}
// 	}

// 	if _, err := parser.ParseStatements(statements); err != nil {
// 		return &InvalidOTTLExpressionError{Err: fmt.Errorf("invalid statement(s) in Transform spec: %w", err)}
// 	}

// 	return nil
// }

func (v *Validator) validateTransformSpec(ctx context.Context, transforms []telemetryv1alpha1.TransformSpec) error {
	telemetrySettings := component.TelemetrySettings{
		Logger: zap.New(zapcore.NewNopCore()),
	}

	functionsMap := ottl.CreateFactoryMap(transformprocessor.DefaultLogFunctions()...)
	parserCollection, err := NewLogParserCollection(telemetrySettings, WithLogParser(functionsMap))
	if err != nil {
		logf.FromContext(ctx).Error(err, "Failed to create OTTL log parser collection")
		return nil
	}

	for _, ts := range transforms {
		contextStatements := convertTransformSpecToContextStatement(ts)
		if _, err := parserCollection.ParseContextStatements(contextStatements); err != nil {
			return &InvalidOTTLExpressionError{Err: fmt.Errorf("invalid Transform spec: %w", err)}
		}

	}

	return nil
}

func convertTransformSpecToContextStatement(transformSpec telemetryv1alpha1.TransformSpec) ContextStatements {
	return ContextStatements{
		Conditions: transformSpec.Conditions,
		Statements: transformSpec.Statements,
		ErrorMode:  ottl.IgnoreError,
	}
}
