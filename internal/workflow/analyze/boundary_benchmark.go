package analyze

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"time"
)

const BoundaryBenchmarkVersion = 1

type BoundaryBenchmarkSuite struct {
	Version int                     `json:"version"`
	Cases   []BoundaryBenchmarkCase `json:"cases"`
}

type BoundaryBenchmarkCase struct {
	ID                string                              `json:"id"`
	Pages             []BoundaryBenchmarkPage             `json:"pages"`
	ExpectedQuestions []BoundaryBenchmarkExpectedQuestion `json:"expected_questions"`
}

type BoundaryBenchmarkPage struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
}

type BoundaryBenchmarkExpectedQuestion struct {
	Label       string `json:"label"`
	SourcePages []int  `json:"source_pages"`
	Status      string `json:"status,omitempty"`
}

type BoundaryBenchmarkReport struct {
	Version     int                      `json:"version"`
	Model       string                   `json:"model"`
	RunsPerCase int                      `json:"runs_per_case"`
	CaseCount   int                      `json:"case_count"`
	DurationMS  int64                    `json:"duration_ms"`
	Summary     BoundaryBenchmarkSummary `json:"summary"`
	Runs        []BoundaryBenchmarkRun   `json:"runs"`
}

type BoundaryBenchmarkSummary struct {
	AttemptedRuns         int      `json:"attempted_runs"`
	SuccessfulRequests    int      `json:"successful_requests"`
	RequestSuccessRate    *float64 `json:"request_success_rate"`
	SchemaValidRuns       int      `json:"schema_valid_runs"`
	SchemaValidRate       *float64 `json:"schema_valid_rate"`
	ExactGroupingRuns     int      `json:"exact_grouping_runs"`
	ExactGroupingRate     *float64 `json:"exact_grouping_rate"`
	LabelMatches          int      `json:"label_matches"`
	LabelChecks           int      `json:"label_checks"`
	LabelAccuracy         *float64 `json:"label_accuracy"`
	StatusMatches         int      `json:"status_matches"`
	StatusChecks          int      `json:"status_checks"`
	StatusAccuracy        *float64 `json:"status_accuracy"`
	StableCases           int      `json:"stable_cases"`
	StabilityRate         *float64 `json:"stability_rate"`
	MeanLatencyMS         *float64 `json:"mean_latency_ms"`
	MedianLatencyMS       *float64 `json:"median_latency_ms"`
	Percentile95LatencyMS *float64 `json:"percentile_95_latency_ms"`
}

type BoundaryBenchmarkRun struct {
	CaseID            string                              `json:"case_id"`
	Run               int                                 `json:"run"`
	LatencyMS         int64                               `json:"latency_ms"`
	RequestSucceeded  bool                                `json:"request_succeeded"`
	SchemaValid       bool                                `json:"schema_valid"`
	GroupingExact     bool                                `json:"grouping_exact"`
	LabelMatches      int                                 `json:"label_matches"`
	LabelChecks       int                                 `json:"label_checks"`
	StatusMatches     int                                 `json:"status_matches"`
	StatusChecks      int                                 `json:"status_checks"`
	ObservedQuestions []BoundaryBenchmarkObservedQuestion `json:"observed_questions,omitempty"`
	ErrorKind         string                              `json:"error_kind,omitempty"`
	Error             string                              `json:"error,omitempty"`
}

type BoundaryBenchmarkObservedQuestion struct {
	Label              string  `json:"label"`
	SourcePages        []int   `json:"source_pages"`
	Boundary           string  `json:"boundary"`
	BoundaryConfidence float64 `json:"boundary_confidence"`
	Status             string  `json:"status"`
}

func DecodeBoundaryBenchmarkSuite(r io.Reader) (BoundaryBenchmarkSuite, error) {
	var suite BoundaryBenchmarkSuite
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&suite); err != nil {
		return BoundaryBenchmarkSuite{}, fmt.Errorf("decode boundary benchmark suite: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return BoundaryBenchmarkSuite{}, errors.New("decode boundary benchmark suite: trailing content")
	}
	if err := ValidateBoundaryBenchmarkSuite(suite); err != nil {
		return BoundaryBenchmarkSuite{}, err
	}
	return suite, nil
}

func ValidateBoundaryBenchmarkSuite(suite BoundaryBenchmarkSuite) error {
	if suite.Version != BoundaryBenchmarkVersion {
		return fmt.Errorf("boundary benchmark version %d is unsupported; want %d", suite.Version, BoundaryBenchmarkVersion)
	}
	if len(suite.Cases) == 0 {
		return errors.New("boundary benchmark suite has no cases")
	}
	caseIDs := make(map[string]struct{}, len(suite.Cases))
	for i, benchmarkCase := range suite.Cases {
		if benchmarkCase.ID == "" || benchmarkCase.ID != strings.TrimSpace(benchmarkCase.ID) {
			return fmt.Errorf("boundary benchmark case %d has an empty or padded id", i+1)
		}
		if _, duplicate := caseIDs[benchmarkCase.ID]; duplicate {
			return fmt.Errorf("boundary benchmark case id %q is duplicated", benchmarkCase.ID)
		}
		caseIDs[benchmarkCase.ID] = struct{}{}
		if err := validateBoundaryBenchmarkCase(benchmarkCase); err != nil {
			return fmt.Errorf("boundary benchmark case %q: %w", benchmarkCase.ID, err)
		}
	}
	return nil
}

func validateBoundaryBenchmarkCase(benchmarkCase BoundaryBenchmarkCase) error {
	if len(benchmarkCase.Pages) == 0 {
		return errors.New("pages are required")
	}
	pageNumbers := make([]int, 0, len(benchmarkCase.Pages))
	seenPages := make(map[int]struct{}, len(benchmarkCase.Pages))
	for _, page := range benchmarkCase.Pages {
		if page.Number < 1 || page.Number > 10000 {
			return fmt.Errorf("page number %d is outside [1,10000]", page.Number)
		}
		if _, duplicate := seenPages[page.Number]; duplicate {
			return fmt.Errorf("page number %d is duplicated", page.Number)
		}
		seenPages[page.Number] = struct{}{}
		pageNumbers = append(pageNumbers, page.Number)
	}
	sort.Ints(pageNumbers)
	if len(benchmarkCase.ExpectedQuestions) == 0 {
		return errors.New("expected_questions are required")
	}
	coveredPages := make([]int, 0, len(pageNumbers))
	for i, question := range benchmarkCase.ExpectedQuestions {
		if len(question.SourcePages) == 0 {
			return fmt.Errorf("expected question %d has no source_pages", i+1)
		}
		if question.Status != "" && question.Status != "detected" && question.Status != "needs review" {
			return fmt.Errorf("expected question %d has invalid status %q", i+1, question.Status)
		}
		coveredPages = append(coveredPages, question.SourcePages...)
	}
	if !slices.Equal(coveredPages, pageNumbers) {
		return fmt.Errorf("expected question pages %v must cover ordered source pages %v exactly once", coveredPages, pageNumbers)
	}
	return nil
}

func (s *Service) RunBoundaryBenchmark(
	ctx context.Context,
	model string,
	suite BoundaryBenchmarkSuite,
	runsPerCase int,
) (BoundaryBenchmarkReport, error) {
	if err := ValidateBoundaryBenchmarkSuite(suite); err != nil {
		return BoundaryBenchmarkReport{}, err
	}
	if model == "" || model != strings.TrimSpace(model) {
		return BoundaryBenchmarkReport{}, errors.New("boundary benchmark model is empty or padded")
	}
	if runsPerCase < 1 || runsPerCase > 100 {
		return BoundaryBenchmarkReport{}, fmt.Errorf("boundary benchmark runs must be within [1,100], got %d", runsPerCase)
	}
	if s.questionProvider == nil {
		return BoundaryBenchmarkReport{}, errors.New("boundary benchmark question provider is not configured")
	}
	report := BoundaryBenchmarkReport{
		Version:     BoundaryBenchmarkVersion,
		Model:       model,
		RunsPerCase: runsPerCase,
		CaseCount:   len(suite.Cases),
		Runs:        make([]BoundaryBenchmarkRun, 0, len(suite.Cases)*runsPerCase),
	}
	started := time.Now()
	for _, benchmarkCase := range suite.Cases {
		pages := boundaryBenchmarkPages(benchmarkCase.Pages)
		for run := 1; run <= runsPerCase; run++ {
			if err := ctx.Err(); err != nil {
				return BoundaryBenchmarkReport{}, err
			}
			runResult := BoundaryBenchmarkRun{CaseID: benchmarkCase.ID, Run: run}
			requestStarted := time.Now()
			questions, err := s.groupAnswerPages(ctx, model, pages)
			runResult.LatencyMS = time.Since(requestStarted).Milliseconds()
			if err != nil {
				if ctxErr := ctx.Err(); ctxErr != nil {
					return BoundaryBenchmarkReport{}, ctxErr
				}
				runResult.Error = err.Error()
				var requestErr *answerBoundaryRequestError
				if errors.As(err, &requestErr) {
					runResult.ErrorKind = "request"
				} else {
					runResult.RequestSucceeded = true
					runResult.ErrorKind = "schema"
				}
				report.Runs = append(report.Runs, runResult)
				continue
			}
			runResult.RequestSucceeded = true
			runResult.SchemaValid = true
			runResult.ObservedQuestions = boundaryBenchmarkObservedQuestions(questions)
			runResult.GroupingExact = boundaryGroupingExact(benchmarkCase.ExpectedQuestions, runResult.ObservedQuestions)
			runResult.LabelMatches, runResult.LabelChecks = boundaryLabelScore(benchmarkCase.ExpectedQuestions, runResult.ObservedQuestions)
			runResult.StatusMatches, runResult.StatusChecks = boundaryStatusScore(benchmarkCase.ExpectedQuestions, runResult.ObservedQuestions)
			report.Runs = append(report.Runs, runResult)
		}
	}
	report.DurationMS = time.Since(started).Milliseconds()
	report.Summary = summarizeBoundaryBenchmark(report.Runs, suite.Cases, runsPerCase)
	return report, nil
}

func boundaryBenchmarkPages(pages []BoundaryBenchmarkPage) []Page {
	converted := make([]Page, 0, len(pages))
	for _, page := range pages {
		converted = append(converted, Page{Number: page.Number, Name: fmt.Sprintf("page-%d", page.Number), Text: page.Text})
	}
	return converted
}

func boundaryBenchmarkObservedQuestions(questions []Question) []BoundaryBenchmarkObservedQuestion {
	observed := make([]BoundaryBenchmarkObservedQuestion, 0, len(questions))
	for _, question := range questions {
		observed = append(observed, BoundaryBenchmarkObservedQuestion{
			Label:              question.Label,
			SourcePages:        append([]int(nil), question.SourcePages...),
			Boundary:           question.Boundary,
			BoundaryConfidence: question.BoundaryConfidence,
			Status:             question.Status,
		})
	}
	return observed
}

func boundaryGroupingExact(expected []BoundaryBenchmarkExpectedQuestion, observed []BoundaryBenchmarkObservedQuestion) bool {
	if len(expected) != len(observed) {
		return false
	}
	for i := range expected {
		if !slices.Equal(expected[i].SourcePages, observed[i].SourcePages) {
			return false
		}
	}
	return true
}

func boundaryLabelScore(expected []BoundaryBenchmarkExpectedQuestion, observed []BoundaryBenchmarkObservedQuestion) (int, int) {
	matches := 0
	checks := 0
	for _, want := range expected {
		if want.Label == "" {
			continue
		}
		checks++
		for _, got := range observed {
			if slices.Equal(want.SourcePages, got.SourcePages) && want.Label == got.Label {
				matches++
				break
			}
		}
	}
	return matches, checks
}

func boundaryStatusScore(expected []BoundaryBenchmarkExpectedQuestion, observed []BoundaryBenchmarkObservedQuestion) (int, int) {
	matches := 0
	checks := 0
	for _, want := range expected {
		if want.Status == "" {
			continue
		}
		checks++
		for _, got := range observed {
			if slices.Equal(want.SourcePages, got.SourcePages) && want.Status == got.Status {
				matches++
				break
			}
		}
	}
	return matches, checks
}

func summarizeBoundaryBenchmark(
	runs []BoundaryBenchmarkRun,
	cases []BoundaryBenchmarkCase,
	runsPerCase int,
) BoundaryBenchmarkSummary {
	summary := BoundaryBenchmarkSummary{AttemptedRuns: len(runs)}
	latencies := make([]int64, 0, len(runs))
	outputsByCase := make(map[string]map[string]struct{}, len(cases))
	validRunsByCase := make(map[string]int, len(cases))
	for _, run := range runs {
		latencies = append(latencies, run.LatencyMS)
		if run.RequestSucceeded {
			summary.SuccessfulRequests++
		}
		if !run.SchemaValid {
			continue
		}
		summary.SchemaValidRuns++
		if run.GroupingExact {
			summary.ExactGroupingRuns++
		}
		summary.LabelMatches += run.LabelMatches
		summary.LabelChecks += run.LabelChecks
		summary.StatusMatches += run.StatusMatches
		summary.StatusChecks += run.StatusChecks
		fingerprint, _ := json.Marshal(run.ObservedQuestions)
		if outputsByCase[run.CaseID] == nil {
			outputsByCase[run.CaseID] = map[string]struct{}{}
		}
		outputsByCase[run.CaseID][string(fingerprint)] = struct{}{}
		validRunsByCase[run.CaseID]++
	}
	for _, benchmarkCase := range cases {
		if validRunsByCase[benchmarkCase.ID] == runsPerCase && len(outputsByCase[benchmarkCase.ID]) == 1 {
			summary.StableCases++
		}
	}
	summary.RequestSuccessRate = boundaryRate(summary.SuccessfulRequests, summary.AttemptedRuns)
	summary.SchemaValidRate = boundaryRate(summary.SchemaValidRuns, summary.SuccessfulRequests)
	summary.ExactGroupingRate = boundaryRate(summary.ExactGroupingRuns, summary.AttemptedRuns)
	summary.LabelAccuracy = boundaryRate(summary.LabelMatches, summary.LabelChecks)
	summary.StatusAccuracy = boundaryRate(summary.StatusMatches, summary.StatusChecks)
	summary.StabilityRate = boundaryRate(summary.StableCases, len(cases))
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		var total int64
		for _, latency := range latencies {
			total += latency
		}
		mean := float64(total) / float64(len(latencies))
		median := boundaryPercentile(latencies, 0.5)
		percentile95 := boundaryPercentile(latencies, 0.95)
		summary.MeanLatencyMS = &mean
		summary.MedianLatencyMS = &median
		summary.Percentile95LatencyMS = &percentile95
	}
	return summary
}

func boundaryRate(numerator int, denominator int) *float64 {
	if denominator == 0 {
		return nil
	}
	rate := float64(numerator) / float64(denominator)
	return &rate
}

func boundaryPercentile(sortedValues []int64, percentile float64) float64 {
	if len(sortedValues) == 1 {
		return float64(sortedValues[0])
	}
	position := percentile * float64(len(sortedValues)-1)
	lower := int(position)
	upper := min(lower+1, len(sortedValues)-1)
	weight := position - float64(lower)
	return float64(sortedValues[lower])*(1-weight) + float64(sortedValues[upper])*weight
}
