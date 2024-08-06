package stubs

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
)

type DaemonSetProber struct {
	err error
}

func NewDaemonSetProber(err error) *DaemonSetProber {
	return &DaemonSetProber{
		err: err,
	}
}

func (d *DaemonSetProber) IsReady(ctx context.Context, name types.NamespacedName) error {
	return d.err
}
