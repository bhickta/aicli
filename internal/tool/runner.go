package tool

import (
	"context"
	"os/exec"
	"strings"
)

type Runner interface {
	CombinedOutput(ctx context.Context, command string, args ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) CombinedOutput(ctx context.Context, command string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, command, args...).CombinedOutput()
}

func (ExecRunner) Start(ctx context.Context, command string, args ...string) error {
	cmd := exec.CommandContext(ctx, command, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		_ = cmd.Wait()
	}()
	return nil
}

func (ExecRunner) CombinedOutputWithInput(ctx context.Context, command string, stdin string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdin = strings.NewReader(stdin)
	return cmd.CombinedOutput()
}
