package whisper

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bhickta/aicli/internal/tool"
)

//go:embed faster_whisper_batch.py
var fasterWhisperBatchScript string

type Request struct {
	Command    string
	AudioPath  string
	OutputBase string
	Model      string
	Device     string
	SRT        bool
	Text       bool
}

type FasterBatchRequest struct {
	PythonCommand string
	AudioPaths    []string
	OutputDir     string
	Model         string
	Device        string
	Workers       int
	BatchSize     int
	BeamSize      int
}

func Run(ctx context.Context, runner tool.Runner, req Request) ([]byte, error) {
	command, err := resolveCommand(runner, req.Command)
	if err != nil {
		return nil, err
	}
	return runner.CombinedOutput(ctx, command, argsFor(command, req)...)
}

func CanRunFasterBatch(runner tool.Runner) bool {
	_, ok := runner.(tool.ExecRunner)
	return ok
}

func RunFasterBatch(ctx context.Context, runner tool.Runner, req FasterBatchRequest) ([]byte, error) {
	command := strings.TrimSpace(req.PythonCommand)
	if command == "" {
		command = "python3"
	}
	if _, ok := runner.(tool.ExecRunner); ok {
		if _, err := exec.LookPath(command); err != nil {
			return nil, err
		}
	}
	return runner.CombinedOutput(ctx, command, fasterBatchArgs(req)...)
}

func FasterBatchUnavailable(out []byte, err error) bool {
	if err == nil {
		return false
	}
	message := string(out)
	return strings.Contains(message, "No module named 'faster_whisper'") ||
		strings.Contains(message, "ModuleNotFoundError")
}

func resolveCommand(runner tool.Runner, configured string) (string, error) {
	command := strings.TrimSpace(configured)
	if command == "" {
		command = "whisper"
	}
	if _, ok := runner.(tool.ExecRunner); !ok {
		return command, nil
	}
	if _, err := exec.LookPath(command); err == nil {
		return command, nil
	}
	if filepath.Base(command) == "whisper-cli" {
		if fallback, err := exec.LookPath("whisper"); err == nil {
			return fallback, nil
		}
	}
	return "", fmt.Errorf("%q was not found in PATH; install whisper.cpp's whisper-cli or set tools.whisper_cli to the full path of your Whisper command, for example /home/bhickta/.local/bin/whisper", command)
}

func argsFor(command string, req Request) []string {
	if isPythonWhisper(command) {
		return pythonArgs(req)
	}
	return whisperCPPArgs(req)
}

func fasterBatchArgs(req FasterBatchRequest) []string {
	model := req.Model
	if model == "" {
		model = "large-v3"
	}
	device := pythonDevice(req.Device)
	workers := req.Workers
	if workers <= 0 {
		workers = 2
	}
	batchSize := req.BatchSize
	if batchSize <= 0 {
		batchSize = 24
	}
	beamSize := req.BeamSize
	if beamSize <= 0 {
		beamSize = 1
	}
	args := []string{
		"-c", fasterWhisperBatchScript,
		"--model", model,
		"--device", device,
		"--compute-type", "float16",
		"--workers", strconv.Itoa(workers),
		"--batch-size", strconv.Itoa(batchSize),
		"--beam-size", strconv.Itoa(beamSize),
		"--output-dir", req.OutputDir,
	}
	args = append(args, req.AudioPaths...)
	return args
}

func isPythonWhisper(command string) bool {
	base := filepath.Base(command)
	return base == "whisper"
}

func pythonArgs(req Request) []string {
	model := req.Model
	if model == "" {
		model = "large-v3"
	}
	outputDir := filepath.Dir(req.OutputBase)
	format := "all"
	switch {
	case req.SRT && !req.Text:
		format = "srt"
	case req.Text && !req.SRT:
		format = "txt"
	}
	return []string{
		req.AudioPath,
		"--model", model,
		"--device", pythonDevice(req.Device),
		"--output_format", format,
		"--output_dir", outputDir,
	}
}

func pythonDevice(device string) string {
	device = strings.TrimSpace(device)
	if device == "" {
		return "cuda"
	}
	return device
}

func whisperCPPArgs(req Request) []string {
	if strings.TrimSpace(req.AudioPath) == "" || strings.TrimSpace(req.OutputBase) == "" {
		return nil
	}
	args := []string{}
	if req.Model != "" {
		args = append(args, "-m", req.Model)
	}
	args = append(args, "-f", req.AudioPath)
	if req.SRT {
		args = append(args, "-osrt")
	}
	if req.Text {
		args = append(args, "-otxt")
	}
	args = append(args, "-of", req.OutputBase)
	return args
}

func OutputError(out []byte, err error) error {
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(string(out))
	if message == "" {
		return err
	}
	return errors.New(message + ": " + err.Error())
}
