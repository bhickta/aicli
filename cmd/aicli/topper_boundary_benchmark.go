package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bhickta/aicli/internal/config"
	openaiProvider "github.com/bhickta/aicli/internal/provider/openai"
	"github.com/bhickta/aicli/internal/workflow/analyze"
)

func runTopperBoundaryBenchmark(args []string) int {
	return runTopperBoundaryBenchmarkIO(args, os.Stdout, os.Stderr)
}

type topperBoundaryBenchmarkOptions struct {
	inputPath string
	model     string
	baseURL   string
	apiKey    string
	runs      int
	timeout   time.Duration
}

func runTopperBoundaryBenchmarkIO(args []string, stdout io.Writer, stderr io.Writer) int {
	options, err := parseTopperBoundaryBenchmarkOptions(args, stderr)
	if err != nil {
		return 2
	}
	if err := executeTopperBoundaryBenchmark(options, stdout); err != nil {
		fmt.Fprintf(stderr, "topper boundary benchmark failed: %v\n", err)
		return 1
	}
	return 0
}

func parseTopperBoundaryBenchmarkOptions(args []string, stderr io.Writer) (topperBoundaryBenchmarkOptions, error) {
	var options topperBoundaryBenchmarkOptions
	defaultBaseURL := strings.TrimSpace(os.Getenv("AICLI_LMS_BASE_URL"))
	if defaultBaseURL == "" {
		defaultBaseURL = "http://127.0.0.1:1234/v1"
	}
	defaultAPIKey := os.Getenv("AICLI_LMS_API_KEY")
	if defaultAPIKey == "" {
		defaultAPIKey = "lms"
	}

	fs := flag.NewFlagSet("aicli topper-boundary-benchmark", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&options.inputPath, "input", "", "strict human-annotated benchmark suite JSON")
	fs.StringVar(&options.model, "model", "", "exact loaded LM Studio model identifier")
	fs.StringVar(&options.baseURL, "base-url", defaultBaseURL, "LM Studio OpenAI-compatible base URL")
	fs.StringVar(&options.apiKey, "api-key", defaultAPIKey, "LM Studio API key")
	fs.IntVar(&options.runs, "runs", 3, "repeated runs per benchmark case (1-100)")
	fs.DurationVar(&options.timeout, "timeout", 2*time.Minute, "timeout for each LM Studio HTTP request")
	if err := fs.Parse(args); err != nil {
		return topperBoundaryBenchmarkOptions{}, err
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "topper-boundary-benchmark does not accept positional arguments")
		return topperBoundaryBenchmarkOptions{}, errors.New("unexpected positional arguments")
	}
	if options.inputPath == "" {
		fmt.Fprintln(stderr, "missing -input")
		return topperBoundaryBenchmarkOptions{}, errors.New("missing input")
	}
	if options.model == "" || options.model != strings.TrimSpace(options.model) {
		fmt.Fprintln(stderr, "-model must be an exact non-empty LM Studio model identifier")
		return topperBoundaryBenchmarkOptions{}, errors.New("invalid model")
	}
	if options.baseURL == "" || options.baseURL != strings.TrimSpace(options.baseURL) {
		fmt.Fprintln(stderr, "-base-url must be non-empty and contain no surrounding whitespace")
		return topperBoundaryBenchmarkOptions{}, errors.New("invalid base URL")
	}
	if options.timeout <= 0 {
		fmt.Fprintln(stderr, "-timeout must be positive")
		return topperBoundaryBenchmarkOptions{}, errors.New("invalid timeout")
	}
	return options, nil
}

func executeTopperBoundaryBenchmark(options topperBoundaryBenchmarkOptions, stdout io.Writer) error {
	input, err := os.Open(options.inputPath)
	if err != nil {
		return fmt.Errorf("open benchmark suite: %w", err)
	}
	defer input.Close()
	suite, err := analyze.DecodeBoundaryBenchmarkSuite(input)
	if err != nil {
		return fmt.Errorf("decode benchmark suite: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client := &http.Client{Timeout: options.timeout}
	localProvider := openaiProvider.NewCompatible(config.ProviderConfig{
		ID:      "lms",
		Type:    "openai-compatible",
		Name:    "LM Studio",
		BaseURL: options.baseURL,
		APIKey:  options.apiKey,
	}, client)
	loadedModels, err := localProvider.ListLoadedModels(ctx)
	if err != nil {
		return fmt.Errorf("preflight loaded LM Studio models: %w", err)
	}
	loaded := false
	loadedIDs := make([]string, 0, len(loadedModels))
	for _, candidate := range loadedModels {
		loadedIDs = append(loadedIDs, candidate.ID)
		if candidate.ID == options.model {
			loaded = true
		}
	}
	if !loaded {
		return fmt.Errorf("model %q is not loaded in LM Studio; loaded models: %s", options.model, strings.Join(loadedIDs, ", "))
	}

	service := analyze.New(config.ToolConfig{}, nil, localProvider, analyze.WithQuestionProvider(localProvider))
	report, err := service.RunBoundaryBenchmark(ctx, options.model, suite, options.runs)
	if err != nil {
		return fmt.Errorf("run boundary benchmark: %w", err)
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("encode boundary benchmark report: %w", err)
	}
	return nil
}
