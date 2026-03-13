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
