package stubs

import (
	"context"
)

type IstioStatusChecker struct {
	IsActive bool
}

func (i *IstioStatusChecker) IsIstioActive(ctx context.Context) (bool, error) {
	return i.IsActive, nil
}
