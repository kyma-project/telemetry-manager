package stubs

import (
	"context"

	"github.com/kyma-project/telemetry-manager/internal/overrides"
)

type OverridesHandler struct{}

func NewOverridesHandler() *OverridesHandler {
	return &OverridesHandler{}
}

func (o *OverridesHandler) LoadOverrides(_ context.Context) (*overrides.Config, error) {
	return &overrides.Config{}, nil
}
