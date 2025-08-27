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
	if err := v.validateTransformSpec(ctx, pipeline.Spec.Transforms); err != nil {
		return err
	}

	return nil
}

func (v *Validator) validateTransformSpec(ctx context.Context, transforms []telemetryv1alpha1.TransformSpec) error {
	telemetrySettings := component.TelemetrySettings{
		Logger: zap.New(zapcore.NewNopCore()),
	}

	functionsMap := ottl.CreateFactoryMap(transformprocessor.DefaultLogFunctions()...)
	parserCollection, err := newGenericParserCollection(telemetrySettings, withLogParser(functionsMap))
	if err != nil {
		logf.FromContext(ctx).Error(err, "Failed to create OTTL log parser collection")
		return nil
	}

	for _, ts := range transforms {
		if _, err := parserCollection.parseStatementsWithConditions(ts.Statements, ts.Conditions); err != nil {
			return &InvalidOTTLExpressionError{Err: fmt.Errorf("invalid Transform spec: %w", err)}
		}

	}

	return nil
}
