package stubs

import (
	"context"
	"k8s.io/apimachinery/pkg/types"
)

type DeploymentSetProber struct {
	err error
}

func NewDeploymentSetProber(err error) *DeploymentSetProber {
	return &DeploymentSetProber{
		err: err,
	}
}

func (d *DeploymentSetProber) IsReady(ctx context.Context, name types.NamespacedName) error {
	return d.err
}
