package tool

import (
	"context"
	"os/exec"
)

type Runner interface {
	CombinedOutput(ctx context.Context, command string, args ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) CombinedOutput(ctx context.Context, command string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, command, args...).CombinedOutput()
}
