package stubs

import (
	"context"

	"github.com/kyma-project/telemetry-manager/internal/overrides"
)

type OverridesHandler struct {
	Paused bool
	Err    error
}

func (o *OverridesHandler) LoadOverrides(_ context.Context) (*overrides.Config, error) {
	if o.Err != nil {
		return nil, o.Err
	}

	return &overrides.Config{
		Metrics: overrides.MetricConfig{Paused: o.Paused},
	}, nil
}
