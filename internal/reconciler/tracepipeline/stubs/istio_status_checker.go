package stubs

import (
	"context"
)

type IstioStatusChecker struct {
	IsActive bool
}

func (i *IstioStatusChecker) IsIstioActive(ctx context.Context) bool {
	return i.IsActive
}
