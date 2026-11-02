package stubs

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type VpaStatusChecker struct {
	CRDExists bool
}

func (c *VpaStatusChecker) VpaCRDExists(ctx context.Context, client client.Client) (bool, error) {
	return c.CRDExists, nil
}

// VpaStatusCheckerWithErr always returns an error from VpaCRDExists.
type VpaStatusCheckerWithErr struct {
	Err error
}

func (c *VpaStatusCheckerWithErr) VpaCRDExists(ctx context.Context, client client.Client) (bool, error) {
	return false, c.Err
}
