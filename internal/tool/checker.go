package tool

import (
	"context"
	"os/exec"
	"time"
)

type Checker struct{}

type Status struct {
	Name      string `json:"name"`
	Command   string `json:"command"`
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (Checker) Check(ctx context.Context, name string, command string, args ...string) Status {
	status := Status{Name: name, Command: command}
	if command == "" {
		status.Error = "command is not configured"
		return status
	}

	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(checkCtx, command, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Available = true
	status.Version = firstLine(string(out))
	return status
}

func firstLine(value string) string {
	for i, r := range value {
		if r == '\n' || r == '\r' {
			return value[:i]
		}
	}
	return value
}
