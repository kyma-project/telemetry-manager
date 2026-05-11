package stubs

import "context"

type IstioStatusChecker struct {
	IsActive bool
}

func (i *IstioStatusChecker) IsIstioActive(_ context.Context) (bool, error) {
	return i.IsActive, nil
}
