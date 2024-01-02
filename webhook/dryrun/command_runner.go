package dryrun

import (
	"context"
	"os/exec"
)

//go:generate mockery --name commandRunner --filename command_runner.go --exported
type commandRunner interface {
	Run(ctx context.Context, command string, args ...string) ([]byte, error)
}

type commandRunnerImpl struct{}

func (r *commandRunnerImpl) Run(ctx context.Context, command string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, command, args...).CombinedOutput()
}
