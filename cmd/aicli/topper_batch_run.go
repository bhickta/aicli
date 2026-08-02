package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/storage"
	"github.com/bhickta/aicli/internal/workflow/analyze"
)

const topperBatchManifestVersion = 1

type topperBatchOptions struct {
	manifestPath        string
	outputDir           string
	serverURL           string
	lanePrefix          string
	ocrModel            string
	reconciliationModel string
	forceOCR            bool
	pollInterval        time.Duration
	timeout             time.Duration
}

type topperBatchManifest struct {
	Version int               `json:"version"`
	Copies  []topperBatchCopy `json:"copies"`
}

type topperBatchCopy struct {
	SourceID string `json:"source_id"`
	Path     string `json:"path"`
}

type topperBatchResult struct {
	Version         int                     `json:"version"`
	StartedAt       time.Time               `json:"started_at"`
	FinishedAt      time.Time               `json:"finished_at"`
	DurationSeconds int64                   `json:"duration_seconds"`
	Total           int                     `json:"total"`
	Completed       int                     `json:"completed"`
	Failed          int                     `json:"failed"`
	Items           []topperBatchItemResult `json:"items"`
}

type topperBatchItemResult struct {
	SourceID      string `json:"source_id"`
	Path          string `json:"path"`
	Lane          string `json:"lane"`
	JobID         string `json:"job_id,omitempty"`
	ReviewID      string `json:"review_id,omitempty"`
	ReviewPath    string `json:"review_path,omitempty"`
	Status        string `json:"status"`
	PageCount     int    `json:"page_count,omitempty"`
	QuestionCount int    `json:"question_count,omitempty"`
	APICalls      int    `json:"api_calls,omitempty"`
	DurationMS    int64  `json:"duration_ms,omitempty"`
	Error         string `json:"error,omitempty"`
}

type topperBatchActiveJob struct {
	copy      topperBatchCopy
	lane      string
	jobID     string
	startedAt time.Time
}

type topperBatchAPI struct {
	baseURL string
	client  *http.Client
}

func runTopperBatch(args []string) int {
	return runTopperBatchIO(args, os.Stdout, os.Stderr)
}

func runTopperBatchIO(args []string, stdout, stderr io.Writer) int {
	options, err := parseTopperBatchOptions(args, stderr)
	if err != nil {
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	result, err := executeTopperBatch(ctx, options, stderr)
	if options.outputDir != "" && result.Total > 0 {
		data, marshalErr := json.MarshalIndent(result, "", "  ")
		if marshalErr == nil {
			data = append(data, '\n')
			marshalErr = writeTopperBatchFileAtomically(
				filepath.Join(options.outputDir, "result.json"),
				data,
			)
		}
		if marshalErr != nil {
			err = errors.Join(err, fmt.Errorf("persist batch result: %w", marshalErr))
		}
	}
	if result.Total > 0 {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if encodeErr := encoder.Encode(result); encodeErr != nil {
			fmt.Fprintf(stderr, "encode topper batch result: %v\n", encodeErr)
			return 1
		}
	}
	if err != nil {
		fmt.Fprintf(stderr, "topper batch failed: %v\n", err)
		return 1
	}
	return 0
}

func parseTopperBatchOptions(args []string, stderr io.Writer) (topperBatchOptions, error) {
	options := topperBatchOptions{}
	defaultServerURL := strings.TrimSpace(os.Getenv("AICLI_TOPPER_SERVER_URL"))
	if defaultServerURL == "" {
		defaultServerURL = "http://127.0.0.1:8765"
	}

	fs := flag.NewFlagSet("aicli topper-batch-run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&options.manifestPath, "manifest", "", "versioned topper batch manifest JSON")
	fs.StringVar(
		&options.outputDir,
		"output-dir",
		"",
		"durable directory for exact review outputs and result.json",
	)
	fs.StringVar(&options.serverURL, "server-url", defaultServerURL, "running AICLI server URL")
	fs.StringVar(&options.lanePrefix, "lane-prefix", "gemini-lane-", "provider ID prefix for independent key lanes")
	fs.StringVar(&options.ocrModel, "ocr-model", "gemini-flash-lite-latest", "direct PDF extraction model")
	fs.StringVar(&options.reconciliationModel, "reconciliation-model", "gemini-3.5-flash", "copy-wide reconciliation model")
	fs.BoolVar(&options.forceOCR, "force-ocr", false, "bypass completed-review reuse for a clean benchmark")
	fs.DurationVar(&options.pollInterval, "poll-interval", 2*time.Second, "job polling interval")
	fs.DurationVar(&options.timeout, "timeout", 45*time.Minute, "whole-batch timeout")
	if err := fs.Parse(args); err != nil {
		return topperBatchOptions{}, err
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "topper-batch-run does not accept positional arguments")
		return topperBatchOptions{}, errors.New("unexpected positional arguments")
	}
	if strings.TrimSpace(options.manifestPath) == "" {
		fmt.Fprintln(stderr, "missing -manifest")
		return topperBatchOptions{}, errors.New("missing manifest")
	}
	if options.outputDir != "" {
		if options.outputDir != strings.TrimSpace(options.outputDir) {
			fmt.Fprintln(stderr, "-output-dir must contain no surrounding whitespace")
			return topperBatchOptions{}, errors.New("invalid output directory")
		}
		absolute, err := filepath.Abs(options.outputDir)
		if err != nil {
			fmt.Fprintf(stderr, "resolve -output-dir: %v\n", err)
			return topperBatchOptions{}, errors.New("invalid output directory")
		}
		options.outputDir = filepath.Clean(absolute)
	}
	parsedURL, err := url.ParseRequestURI(options.serverURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		fmt.Fprintln(stderr, "-server-url must be an absolute HTTP(S) URL")
		return topperBatchOptions{}, errors.New("invalid server URL")
	}
	options.serverURL = strings.TrimRight(options.serverURL, "/")
	if options.lanePrefix == "" || options.lanePrefix != strings.TrimSpace(options.lanePrefix) {
		fmt.Fprintln(stderr, "-lane-prefix must be non-empty and contain no surrounding whitespace")
		return topperBatchOptions{}, errors.New("invalid lane prefix")
	}
	if options.ocrModel == "" || options.ocrModel != strings.TrimSpace(options.ocrModel) {
		fmt.Fprintln(stderr, "-ocr-model must be an exact non-empty model identifier")
		return topperBatchOptions{}, errors.New("invalid OCR model")
	}
	if options.reconciliationModel == "" || options.reconciliationModel != strings.TrimSpace(options.reconciliationModel) {
		fmt.Fprintln(stderr, "-reconciliation-model must be an exact non-empty model identifier")
		return topperBatchOptions{}, errors.New("invalid reconciliation model")
	}
	if options.pollInterval <= 0 || options.timeout <= 0 {
		fmt.Fprintln(stderr, "-poll-interval and -timeout must be positive")
		return topperBatchOptions{}, errors.New("invalid duration")
	}
	return options, nil
}

func executeTopperBatch(ctx context.Context, options topperBatchOptions, stderr io.Writer) (topperBatchResult, error) {
	manifest, err := loadTopperBatchManifest(options.manifestPath)
	if err != nil {
		return topperBatchResult{}, err
	}
	api := topperBatchAPI{
		baseURL: options.serverURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
	if err := api.health(ctx); err != nil {
		return topperBatchResult{}, fmt.Errorf("AICLI server preflight: %w", err)
	}
	lanes, err := api.lanes(ctx, options.lanePrefix)
	if err != nil {
		return topperBatchResult{}, fmt.Errorf("discover provider lanes: %w", err)
	}
	if len(lanes) == 0 {
		return topperBatchResult{}, fmt.Errorf("no provider IDs begin with %q", options.lanePrefix)
	}

	batchCtx, cancel := context.WithTimeout(ctx, options.timeout)
	defer cancel()
	result := topperBatchResult{
		Version:   topperBatchManifestVersion,
		StartedAt: time.Now().UTC(),
		Total:     len(manifest.Copies),
		Items:     make([]topperBatchItemResult, 0, len(manifest.Copies)),
	}
	available := append([]string(nil), lanes...)
	pending := append([]topperBatchCopy(nil), manifest.Copies...)
	active := make(map[string]topperBatchActiveJob, len(lanes))
	ticker := time.NewTicker(options.pollInterval)
	defer ticker.Stop()

	for len(pending) > 0 || len(active) > 0 {
		for len(pending) > 0 && len(available) > 0 {
			copyItem := pending[0]
			pending = pending[1:]
			lane := available[0]
			available = available[1:]
			job, submitErr := api.submit(batchCtx, copyItem, lane, options)
			if submitErr != nil {
				result.Items = append(result.Items, topperBatchItemResult{
					SourceID: copyItem.SourceID, Path: copyItem.Path, Lane: lane,
					Status: storage.JobStatusFailed, Error: submitErr.Error(),
				})
				result.Failed++
				available = append(available, lane)
				continue
			}
			active[job.ID] = topperBatchActiveJob{
				copy: copyItem, lane: lane, jobID: job.ID, startedAt: time.Now(),
			}
			fmt.Fprintf(stderr, "submitted source=%s lane=%s job=%s\n", copyItem.SourceID, lane, job.ID)
		}
		if len(active) == 0 {
			break
		}

		select {
		case <-batchCtx.Done():
			return finishTopperBatchResult(result), batchCtx.Err()
		case <-ticker.C:
		}

		jobIDs := make([]string, 0, len(active))
		for jobID := range active {
			jobIDs = append(jobIDs, jobID)
		}
		sort.Strings(jobIDs)
		for _, jobID := range jobIDs {
			activeJob := active[jobID]
			job, pollErr := api.job(batchCtx, jobID)
			if pollErr != nil {
				fmt.Fprintf(stderr, "poll source=%s job=%s: %v\n", activeJob.copy.SourceID, jobID, pollErr)
				continue
			}
			if !finishedTopperBatchJob(job.Status) {
				continue
			}
			item := topperBatchItemResult{
				SourceID:   activeJob.copy.SourceID,
				Path:       activeJob.copy.Path,
				Lane:       activeJob.lane,
				JobID:      activeJob.jobID,
				Status:     job.Status,
				DurationMS: time.Since(activeJob.startedAt).Milliseconds(),
				Error:      job.Error,
			}
			if job.Status == storage.JobStatusCompleted {
				var review analyze.Response
				if err := json.Unmarshal([]byte(job.Output), &review); err != nil {
					item.Status = storage.JobStatusFailed
					item.Error = fmt.Sprintf("decode completed review: %v", err)
					result.Failed++
				} else if strings.TrimSpace(review.ReviewID) == "" {
					item.Status = storage.JobStatusFailed
					item.Error = "completed review has an empty review_id"
					result.Failed++
				} else {
					item.ReviewID = review.ReviewID
					if options.outputDir != "" {
						item.ReviewPath, err = writeTopperBatchReview(
							options.outputDir,
							activeJob.copy.SourceID,
							[]byte(job.Output),
						)
						if err != nil {
							item.Status = storage.JobStatusFailed
							item.Error = fmt.Sprintf("persist completed review: %v", err)
							result.Failed++
						} else {
							result.Completed++
						}
					} else {
						result.Completed++
					}
					item.PageCount = len(review.Pages)
					item.QuestionCount = len(review.Questions)
					item.APICalls = review.APICalls
				}
			} else {
				result.Failed++
			}
			result.Items = append(result.Items, item)
			delete(active, jobID)
			available = append(available, activeJob.lane)
			fmt.Fprintf(stderr, "finished source=%s lane=%s status=%s duration=%s\n", item.SourceID, item.Lane, item.Status, time.Since(activeJob.startedAt).Round(time.Second))
		}
	}

	result = finishTopperBatchResult(result)
	sort.Slice(result.Items, func(i, j int) bool { return result.Items[i].SourceID < result.Items[j].SourceID })
	if result.Failed > 0 {
		return result, fmt.Errorf("%d of %d copies failed", result.Failed, result.Total)
	}
	return result, nil
}

func writeTopperBatchReview(outputDir, sourceID string, data []byte) (string, error) {
	reviewsDir := filepath.Join(outputDir, "reviews")
	if err := os.MkdirAll(reviewsDir, 0o700); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(sourceID))
	path := filepath.Join(reviewsDir, fmt.Sprintf("review-%x.json", sum[:]))
	existing, err := os.ReadFile(path)
	switch {
	case err == nil:
		if !bytes.Equal(existing, data) {
			return "", fmt.Errorf("immutable output %q already contains a different review", path)
		}
		return path, nil
	case !errors.Is(err, os.ErrNotExist):
		return "", err
	}
	if err := writeTopperBatchFileAtomically(path, data); err != nil {
		return "", err
	}
	return path, nil
}

func writeTopperBatchFileAtomically(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".topper-batch-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func loadTopperBatchManifest(path string) (topperBatchManifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return topperBatchManifest{}, fmt.Errorf("open batch manifest: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var manifest topperBatchManifest
	if err := decoder.Decode(&manifest); err != nil {
		return topperBatchManifest{}, fmt.Errorf("decode batch manifest: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return topperBatchManifest{}, fmt.Errorf("decode batch manifest: %w", err)
	}
	if manifest.Version != topperBatchManifestVersion {
		return topperBatchManifest{}, fmt.Errorf("batch manifest version %d is unsupported; want %d", manifest.Version, topperBatchManifestVersion)
	}
	if len(manifest.Copies) == 0 {
		return topperBatchManifest{}, errors.New("batch manifest contains no copies")
	}
	seenIDs := make(map[string]struct{}, len(manifest.Copies))
	seenPaths := make(map[string]struct{}, len(manifest.Copies))
	for index := range manifest.Copies {
		copyItem := &manifest.Copies[index]
		if copyItem.SourceID == "" || copyItem.SourceID != strings.TrimSpace(copyItem.SourceID) {
			return topperBatchManifest{}, fmt.Errorf("copies[%d].source_id must be non-empty without surrounding whitespace", index)
		}
		if _, exists := seenIDs[copyItem.SourceID]; exists {
			return topperBatchManifest{}, fmt.Errorf("duplicate source_id %q", copyItem.SourceID)
		}
		seenIDs[copyItem.SourceID] = struct{}{}
		absolute, err := filepath.Abs(copyItem.Path)
		if err != nil {
			return topperBatchManifest{}, fmt.Errorf("copies[%d].path: %w", index, err)
		}
		copyItem.Path = filepath.Clean(absolute)
		if !strings.EqualFold(filepath.Ext(copyItem.Path), ".pdf") {
			return topperBatchManifest{}, fmt.Errorf("copies[%d].path is not a PDF", index)
		}
		if _, exists := seenPaths[copyItem.Path]; exists {
			return topperBatchManifest{}, fmt.Errorf("duplicate path %q", copyItem.Path)
		}
		seenPaths[copyItem.Path] = struct{}{}
		info, err := os.Stat(copyItem.Path)
		if err != nil {
			return topperBatchManifest{}, fmt.Errorf("copies[%d].path: %w", index, err)
		}
		if !info.Mode().IsRegular() {
			return topperBatchManifest{}, fmt.Errorf("copies[%d].path is not a regular file", index)
		}
	}
	return manifest, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("unexpected data after JSON document")
}

func finishTopperBatchResult(result topperBatchResult) topperBatchResult {
	result.FinishedAt = time.Now().UTC()
	result.DurationSeconds = int64(result.FinishedAt.Sub(result.StartedAt).Seconds())
	return result
}

func finishedTopperBatchJob(status string) bool {
	return status == storage.JobStatusCompleted || status == storage.JobStatusFailed || status == storage.JobStatusCancelled
}

func (api topperBatchAPI) health(ctx context.Context) error {
	return api.getJSON(ctx, "/api/health", nil)
}

func (api topperBatchAPI) lanes(ctx context.Context, prefix string) ([]string, error) {
	var response struct {
		Providers []config.ProviderConfig `json:"providers"`
	}
	if err := api.getJSON(ctx, "/api/providers", &response); err != nil {
		return nil, err
	}
	lanes := make([]string, 0, len(response.Providers))
	for _, providerConfig := range response.Providers {
		if strings.HasPrefix(providerConfig.ID, prefix) {
			lanes = append(lanes, providerConfig.ID)
		}
	}
	sort.Slice(lanes, func(i, j int) bool { return topperLaneLess(lanes[i], lanes[j], prefix) })
	return lanes, nil
}

func topperLaneLess(left, right, prefix string) bool {
	leftNumber, leftErr := strconv.Atoi(strings.TrimPrefix(left, prefix))
	rightNumber, rightErr := strconv.Atoi(strings.TrimPrefix(right, prefix))
	if leftErr == nil && rightErr == nil {
		return leftNumber < rightNumber
	}
	return left < right
}

func (api topperBatchAPI) submit(ctx context.Context, copyItem topperBatchCopy, lane string, options topperBatchOptions) (storage.Job, error) {
	payload := map[string]any{
		"provider_id":     lane,
		"model":           options.ocrModel,
		"ocr_provider_id": lane,
		"ocr_model":       options.ocrModel,
		"boundary_model":  options.reconciliationModel,
		"path":            copyItem.Path,
		"ocr_input_mode":  analyze.OCRInputModePDFDirect,
		"force_ocr":       options.forceOCR,
		"question_split":  true,
		"unload_models":   false,
	}
	var response struct {
		Job storage.Job `json:"job"`
	}
	if err := api.postJSON(ctx, "/api/workflows/analyze/run", payload, &response); err != nil {
		return storage.Job{}, err
	}
	if response.Job.ID == "" {
		return storage.Job{}, errors.New("analyze endpoint returned an empty job ID")
	}
	return response.Job, nil
}

func (api topperBatchAPI) job(ctx context.Context, jobID string) (storage.Job, error) {
	var job storage.Job
	if err := api.getJSON(ctx, "/api/jobs/"+url.PathEscape(jobID), &job); err != nil {
		return storage.Job{}, err
	}
	return job, nil
}

func (api topperBatchAPI) getJSON(ctx context.Context, path string, output any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, api.baseURL+path, nil)
	if err != nil {
		return err
	}
	return api.doJSON(request, output)
}

func (api topperBatchAPI) postJSON(ctx context.Context, path string, input, output any) error {
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, api.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	return api.doJSON(request, output)
}

func (api topperBatchAPI) doJSON(request *http.Request, output any) error {
	response, err := api.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("%s %s returned %s: %s", request.Method, request.URL.Path, response.Status, strings.TrimSpace(string(body)))
	}
	if output == nil {
		_, err := io.Copy(io.Discard, response.Body)
		return err
	}
	if err := json.NewDecoder(response.Body).Decode(output); err != nil {
		return fmt.Errorf("decode %s %s response: %w", request.Method, request.URL.Path, err)
	}
	return nil
}
