package analyze

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/workflow/document"
)

const directPDFCheckpointVersion = 1

type directPDFChunk struct {
	Index     int `json:"index"`
	FirstPage int `json:"first_page"`
	LastPage  int `json:"last_page"`
}

type directPDFChunkResult struct {
	Chunk    directPDFChunk `json:"chunk"`
	Response Response       `json:"response"`
}

type directPDFChunkCheckpoint struct {
	Version      int                  `json:"version"`
	SourceSHA256 string               `json:"source_sha256"`
	Model        string               `json:"model"`
	PromptSHA256 string               `json:"prompt_sha256"`
	Result       directPDFChunkResult `json:"result"`
}

type directPDFChunkContentError struct {
	Err      error
	Usage    *provider.TokenUsage
	APICalls int
}

func (e *directPDFChunkContentError) Error() string {
	return e.Err.Error()
}

func (e *directPDFChunkContentError) Unwrap() error {
	return e.Err
}

func planDirectPDFChunks(pageCount int) ([]directPDFChunk, error) {
	if pageCount <= 0 {
		return nil, errors.New("direct PDF has no pages")
	}
	if directPDFChunkOverlapPages < 0 || directPDFChunkOverlapPages >= directPDFChunkPages {
		return nil, errors.New("invalid direct PDF chunk overlap")
	}
	step := directPDFChunkPages - directPDFChunkOverlapPages
	chunks := make([]directPDFChunk, 0, (pageCount+step-1)/step)
	for first := 1; first <= pageCount; first += step {
		last := minIntValue(first+directPDFChunkPages-1, pageCount)
		chunks = append(chunks, directPDFChunk{
			Index:     len(chunks),
			FirstPage: first,
			LastPage:  last,
		})
		if last == pageCount {
			break
		}
	}
	return chunks, nil
}

func (s *Service) directPDFChunkedReview(
	ctx context.Context,
	req Request,
	reviewID string,
	sourceData []byte,
	pageCount int,
	processor provider.DocumentProcessor,
) (Response, error) {
	chunks, err := planDirectPDFChunks(pageCount)
	if err != nil {
		return Response{}, err
	}
	model := firstNonBlank(req.OCRModel, req.Model)
	pdfName := filepath.Base(req.Path)
	sourceHash := sha256.Sum256(sourceData)
	sourceSHA256 := hex.EncodeToString(sourceHash[:])
	checkpointDir := s.directPDFCheckpointDir(reviewID, sourceSHA256, model)

	tempDir, err := os.MkdirTemp("", "aicli-direct-pdf-")
	if err != nil {
		return Response{}, err
	}
	defer os.RemoveAll(tempDir)

	s.logInfo(
		"direct PDF chunked extraction started",
		"path", req.Path,
		"model", model,
		"pages", pageCount,
		"chunks", len(chunks),
		"chunk_pages", directPDFChunkPages,
		"overlap_pages", directPDFChunkOverlapPages,
	)
	results := make([]directPDFChunkResult, 0, len(chunks))
	for _, chunk := range chunks {
		if err := ctx.Err(); err != nil {
			return Response{}, err
		}
		prompts := directPDFChunkPrompts(pdfName, pageCount, chunk)
		promptSHA256 := directPDFPromptsSHA256(prompts)
		cached, found, err := loadDirectPDFChunkCheckpoint(
			checkpointDir,
			chunk,
			sourceSHA256,
			model,
			promptSHA256,
		)
		if err != nil {
			return Response{}, err
		}
		if found {
			results = append(results, cached)
			s.logInfo("direct PDF chunk checkpoint reused", "chunk", chunk.Index+1, "first_page", chunk.FirstPage, "last_page", chunk.LastPage)
			continue
		}

		result, err := s.extractDirectPDFChunkWithFallback(ctx, req, pdfName, pageCount, chunk, tempDir, prompts, processor)
		if err != nil {
			return Response{}, fmt.Errorf("extract direct PDF chunk %d pages %d-%d: %w", chunk.Index+1, chunk.FirstPage, chunk.LastPage, err)
		}
		if err := saveDirectPDFChunkCheckpoint(
			checkpointDir,
			directPDFChunkCheckpoint{
				Version:      directPDFCheckpointVersion,
				SourceSHA256: sourceSHA256,
				Model:        model,
				PromptSHA256: promptSHA256,
				Result:       result,
			},
		); err != nil {
			return Response{}, err
		}
		results = append(results, result)
	}

	metadata := mergeDirectPDFChunkMetadata(results)
	pages, pageWarnings := mergeDirectPDFChunkPages(results, pageCount)
	questions, report, reconcileWarnings, reconcileUsage, reconcileCalls, err := s.reconcileDirectPDFChunks(
		ctx,
		model,
		pdfName,
		sourceData,
		pageCount,
		results,
	)
	if err != nil {
		return Response{}, err
	}
	questions, refreshUsage, refreshCalls, err := s.refreshDirectPDFInternalProcessingAnalysis(
		ctx,
		req,
		model,
		tempDir,
		questions,
		processor,
	)
	if err != nil {
		return Response{}, err
	}
	usage, calls := directPDFChunkUsage(results)
	usage = addTokenUsage(usage, reconcileUsage)
	usage = addTokenUsage(usage, refreshUsage)
	calls += reconcileCalls
	calls += refreshCalls
	result := Response{
		Kind:       "topper_copy_review",
		ReviewID:   reviewID,
		PDFName:    pdfName,
		SourceMode: OCRInputModePDFDirect,
		APICalls:   calls,
		Usage:      usage,
		Metadata:   metadata,
		Pages:      pages,
		Questions:  questions,
		Report:     report,
	}
	result.Quality = analysisQuality(result.Pages, result.Questions)
	appendDirectPDFQualityWarnings(result.Quality, append(pageWarnings, reconcileWarnings...))
	s.logInfo(
		"direct PDF chunked extraction completed",
		"pages", len(result.Pages),
		"questions", len(result.Questions),
		"chunks", len(chunks),
		"api_calls", result.APICalls,
	)
	return result, nil
}

func (s *Service) extractDirectPDFChunkWithFallback(
	ctx context.Context,
	req Request,
	pdfName string,
	pageCount int,
	chunk directPDFChunk,
	tempDir string,
	prompts []string,
	processor provider.DocumentProcessor,
) (directPDFChunkResult, error) {
	result, err := s.extractDirectPDFChunk(ctx, req, pdfName, chunk, tempDir, prompts, processor)
	if err == nil {
		return result, nil
	}
	var contentErr *directPDFChunkContentError
	if !errors.As(err, &contentErr) {
		return directPDFChunkResult{}, err
	}
	subchunks, ok := splitDirectPDFChunkForFallback(chunk)
	if !ok {
		return directPDFChunkResult{}, err
	}
	s.logWarn(
		"direct PDF chunk validation failed; retrying smaller overlapping ranges",
		"chunk", chunk.Index+1,
		"first_page", chunk.FirstPage,
		"last_page", chunk.LastPage,
		"error", err,
	)
	results := make([]directPDFChunkResult, 0, len(subchunks))
	for _, subchunk := range subchunks {
		prompts := directPDFChunkPrompts(pdfName, pageCount, subchunk)
		result, subErr := s.extractDirectPDFChunkWithFallback(
			ctx,
			req,
			pdfName,
			pageCount,
			subchunk,
			tempDir,
			prompts,
			processor,
		)
		if subErr != nil {
			return directPDFChunkResult{}, fmt.Errorf(
				"smaller range pages %d-%d after chunk validation failure: %w",
				subchunk.FirstPage,
				subchunk.LastPage,
				subErr,
			)
		}
		results = append(results, result)
	}
	merged, err := mergeDirectPDFFallbackChunk(chunk, pdfName, results)
	if err != nil {
		return directPDFChunkResult{}, err
	}
	merged.Response.Usage = addTokenUsage(contentErr.Usage, merged.Response.Usage)
	merged.Response.APICalls += contentErr.APICalls
	return merged, nil
}

func splitDirectPDFChunkForFallback(chunk directPDFChunk) ([]directPDFChunk, bool) {
	pageCount := chunk.LastPage - chunk.FirstPage + 1
	if pageCount <= 1 {
		return nil, false
	}
	if pageCount == 2 {
		return []directPDFChunk{
			{Index: chunk.Index, FirstPage: chunk.FirstPage, LastPage: chunk.FirstPage},
			{Index: chunk.Index, FirstPage: chunk.LastPage, LastPage: chunk.LastPage},
		}, true
	}
	middle := chunk.FirstPage + pageCount/2
	return []directPDFChunk{
		{Index: chunk.Index, FirstPage: chunk.FirstPage, LastPage: middle},
		{Index: chunk.Index, FirstPage: middle, LastPage: chunk.LastPage},
	}, true
}

func mergeDirectPDFFallbackChunk(
	chunk directPDFChunk,
	pdfName string,
	results []directPDFChunkResult,
) (directPDFChunkResult, error) {
	selectedPages := make(map[int]Page, chunk.LastPage-chunk.FirstPage+1)
	questions := []Question{}
	reports := []string{}
	for _, result := range results {
		for _, page := range result.Response.Pages {
			current, found := selectedPages[page.Number]
			if !found || directPDFPageScore(page) > directPDFPageScore(current) {
				selectedPages[page.Number] = page
			}
		}
		questions = append(questions, result.Response.Questions...)
		if report := strings.TrimSpace(result.Response.Report); report != "" {
			reports = append(reports, report)
		}
	}
	pages := make([]Page, 0, chunk.LastPage-chunk.FirstPage+1)
	for pageNumber := chunk.FirstPage; pageNumber <= chunk.LastPage; pageNumber++ {
		page, found := selectedPages[pageNumber]
		if !found {
			return directPDFChunkResult{}, fmt.Errorf(
				"smaller-range fallback omitted page %d from chunk pages %d-%d",
				pageNumber,
				chunk.FirstPage,
				chunk.LastPage,
			)
		}
		pages = append(pages, page)
	}
	sortQuestions(questions)
	usage, calls := directPDFChunkUsage(results)
	return directPDFChunkResult{
		Chunk: chunk,
		Response: Response{
			Kind:       "topper_copy_chunk",
			PDFName:    pdfName,
			SourceMode: OCRInputModePDFDirect,
			APICalls:   calls,
			Usage:      usage,
			Metadata:   mergeDirectPDFChunkMetadata(results),
			Pages:      pages,
			Questions:  questions,
			Report:     strings.Join(reports, "\n\n"),
		},
	}, nil
}

func (s *Service) extractDirectPDFChunk(
	ctx context.Context,
	req Request,
	pdfName string,
	chunk directPDFChunk,
	tempDir string,
	prompts []string,
	processor provider.DocumentProcessor,
) (directPDFChunkResult, error) {
	chunkPath := filepath.Join(tempDir, fmt.Sprintf("chunk-%03d-p%04d-p%04d.pdf", chunk.Index+1, chunk.FirstPage, chunk.LastPage))
	if err := document.SplitPDFRange(ctx, s.runner, s.tools.QPDF, req.Path, chunkPath, chunk.FirstPage, chunk.LastPage); err != nil {
		return directPDFChunkResult{}, err
	}
	data, err := os.ReadFile(chunkPath)
	if err != nil {
		return directPDFChunkResult{}, err
	}
	model := firstNonBlank(req.OCRModel, req.Model)
	var usage *provider.TokenUsage
	for attempt, prompt := range prompts {
		res, err := processor.Document(ctx, provider.DocumentRequest{
			Model:            model,
			Prompt:           prompt,
			Data:             data,
			MIMEType:         "application/pdf",
			ResponseMIMEType: "application/json",
			Temperature:      0,
			MaxTokens:        geminiLiteDirectPDFMaxTokens,
		})
		usage = addTokenUsage(usage, res.Usage)
		if err != nil {
			return directPDFChunkResult{}, err
		}
		if err := validateDirectPDFFinishReason(res.FinishReason); err != nil {
			return directPDFChunkResult{}, &directPDFChunkContentError{Err: err, Usage: usage, APICalls: attempt + 1}
		}
		metadata, pages, questions, report, err := parseChunkPDFManifest(res.Content)
		if err == nil {
			pages, questions, err = globalizeDirectPDFChunk(chunk, pages, questions)
		}
		if err != nil {
			if attempt+1 < len(prompts) {
				s.logWarn("direct PDF chunk incomplete; retrying", "chunk", chunk.Index+1, "first_page", chunk.FirstPage, "last_page", chunk.LastPage, "error", err)
				continue
			}
			return directPDFChunkResult{}, &directPDFChunkContentError{Err: err, Usage: usage, APICalls: attempt + 1}
		}
		return directPDFChunkResult{
			Chunk: chunk,
			Response: Response{
				Kind:       "topper_copy_chunk",
				PDFName:    pdfName,
				SourceMode: OCRInputModePDFDirect,
				APICalls:   attempt + 1,
				Usage:      usage,
				Metadata:   metadata,
				Pages:      pages,
				Questions:  questions,
				Report:     report,
			},
		}, nil
	}
	return directPDFChunkResult{}, errors.New("direct PDF chunk extraction failed")
}

func directPDFChunkPrompts(pdfName string, pageCount int, chunk directPDFChunk) []string {
	return []string{
		directPDFChunkPrompt(pdfName, pageCount, chunk, false),
		directPDFChunkPrompt(pdfName, pageCount, chunk, true),
	}
}

func directPDFPromptsSHA256(prompts []string) string {
	data, _ := json.Marshal(prompts)
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func directPDFChunkPrompt(pdfName string, pageCount int, chunk directPDFChunk, strict bool) string {
	localPages := chunk.LastPage - chunk.FirstPage + 1
	prefix := fmt.Sprintf(`The attached PDF is chunk %d of a larger %d-page answer-copy.
It contains original global pages %d-%d, represented inside this attachment as local pages 1-%d.
In every pages[].number and questions[].source_pages value, use attachment-local page numbers only. The application will apply the global offset.
Return a pages[] entry for every attached page, including cover, question-paper, evaluation, and blank pages.
Analyze only visible content in this attachment. Preserve answer text and source pages without inventing content beyond the attached pages.
The attachment boundary is an internal processing detail, not evidence that the candidate's answer was truncated or incomplete. Never mention chunks, chunk edges, extraction windows, or technical processing as a content cause in any returned field. Describe only visible page evidence—for example, writing visibly ends, the next visible answer area is blank, or handwriting is unreadable—without inferring technical loss.

`, chunk.Index+1, pageCount, chunk.FirstPage, chunk.LastPage, localPages)
	suffix := `

Chunk-specific output limit: keep report under 1,200 characters and summarize only this chunk. Do not repeat full answer text in report. A later reconciliation call creates the detailed copy-wide report. This limit does not apply to questions[].answer_markdown, which must remain complete for all visible content in the attachment.
`
	return prefix + oneShotPDFPromptBody(pdfName, strict) + suffix
}

func globalizeDirectPDFChunk(chunk directPDFChunk, pages []Page, questions []Question) ([]Page, []Question, error) {
	localPageCount := chunk.LastPage - chunk.FirstPage + 1
	if len(pages) != localPageCount {
		return nil, nil, newIncompleteDirectPDFError(
			"chunk pages %d-%d returned %d page record(s), want %d",
			chunk.FirstPage,
			chunk.LastPage,
			len(pages),
			localPageCount,
		)
	}
	seenPages := make(map[int]bool, localPageCount)
	for index := range pages {
		local := pages[index].Number
		if local < 1 || local > localPageCount || seenPages[local] {
			return nil, nil, newIncompleteDirectPDFError("chunk pages %d-%d returned invalid local page %d", chunk.FirstPage, chunk.LastPage, local)
		}
		seenPages[local] = true
		global := chunk.FirstPage + local - 1
		pages[index].Number = global
		pages[index].Name = fmt.Sprintf("page-%d", global)
	}
	for index := range questions {
		if len(questions[index].SourcePages) == 0 {
			return nil, nil, newIncompleteDirectPDFError("chunk pages %d-%d returned question %q without source pages", chunk.FirstPage, chunk.LastPage, questions[index].Label)
		}
		for pageIndex, local := range questions[index].SourcePages {
			if local < 1 || local > localPageCount {
				return nil, nil, newIncompleteDirectPDFError("chunk pages %d-%d returned question %q with invalid local page %d", chunk.FirstPage, chunk.LastPage, questions[index].Label, local)
			}
			questions[index].SourcePages[pageIndex] = chunk.FirstPage + local - 1
		}
		questions[index].SourcePages = positiveUniqueInts(questions[index].SourcePages)
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Number < pages[j].Number })
	sortQuestions(questions)
	return pages, questions, nil
}

func (s *Service) directPDFCheckpointDir(reviewID string, sourceSHA256 string, model string) string {
	if strings.TrimSpace(s.artifactDir) == "" || strings.TrimSpace(reviewID) == "" {
		return ""
	}
	fingerprint := sha256.Sum256([]byte(fmt.Sprintf(
		"v%d\x00%s\x00%s\x00%d\x00%d",
		directPDFCheckpointVersion,
		sourceSHA256,
		model,
		directPDFChunkPages,
		directPDFChunkOverlapPages,
	)))
	return filepath.Join(
		s.artifactDir,
		"topper-copy",
		filepath.Base(reviewID),
		"direct-pdf",
		hex.EncodeToString(fingerprint[:8]),
	)
}

func directPDFChunkCheckpointPath(dir string, chunk directPDFChunk) string {
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, fmt.Sprintf("chunk-%03d-p%04d-p%04d.json", chunk.Index+1, chunk.FirstPage, chunk.LastPage))
}

func loadDirectPDFChunkCheckpoint(
	dir string,
	chunk directPDFChunk,
	sourceSHA256 string,
	model string,
	promptSHA256 string,
) (directPDFChunkResult, bool, error) {
	path := directPDFChunkCheckpointPath(dir, chunk)
	if path == "" {
		return directPDFChunkResult{}, false, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return directPDFChunkResult{}, false, nil
	}
	if err != nil {
		return directPDFChunkResult{}, false, err
	}
	var checkpoint directPDFChunkCheckpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return directPDFChunkResult{}, false, fmt.Errorf("parse direct PDF checkpoint %s: %w", path, err)
	}
	if checkpoint.Version != directPDFCheckpointVersion ||
		checkpoint.SourceSHA256 != sourceSHA256 ||
		checkpoint.Model != model ||
		checkpoint.PromptSHA256 == "" ||
		checkpoint.PromptSHA256 != promptSHA256 ||
		checkpoint.Result.Chunk != chunk {
		return directPDFChunkResult{}, false, nil
	}
	return checkpoint.Result, true, nil
}

func saveDirectPDFChunkCheckpoint(dir string, checkpoint directPDFChunkCheckpoint) error {
	path := directPDFChunkCheckpointPath(dir, checkpoint.Result.Chunk)
	if path == "" {
		return nil
	}
	if strings.TrimSpace(checkpoint.PromptSHA256) == "" {
		return errors.New("direct PDF checkpoint requires a prompt fingerprint")
	}
	if _, err := os.Stat(path); err == nil {
		_, found, loadErr := loadDirectPDFChunkCheckpoint(
			dir,
			checkpoint.Result.Chunk,
			checkpoint.SourceSHA256,
			checkpoint.Model,
			checkpoint.PromptSHA256,
		)
		if loadErr != nil {
			return loadErr
		}
		if found {
			return nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".chunk-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o644); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func mergeDirectPDFChunkMetadata(results []directPDFChunkResult) *CopyMetadata {
	var merged *CopyMetadata
	for _, result := range results {
		meta := result.Response.Metadata
		if meta == nil {
			continue
		}
		if merged == nil {
			copy := *meta
			copy.Tags = append([]string(nil), meta.Tags...)
			copy.SearchHints = append([]string(nil), meta.SearchHints...)
			copy.Notes = ""
			merged = &copy
			continue
		}
		merged.SuggestedPDFName = firstNonBlank(merged.SuggestedPDFName, meta.SuggestedPDFName)
		merged.TopperName = firstNonBlank(merged.TopperName, meta.TopperName)
		merged.CandidateName = firstNonBlank(merged.CandidateName, meta.CandidateName)
		merged.Rank = firstNonBlank(merged.Rank, meta.Rank)
		merged.Exam = firstNonBlank(merged.Exam, meta.Exam)
		merged.Year = firstNonBlank(merged.Year, meta.Year)
		merged.Paper = firstNonBlank(merged.Paper, meta.Paper)
		merged.Subject = firstNonBlank(merged.Subject, meta.Subject)
		merged.TestSeries = firstNonBlank(merged.TestSeries, meta.TestSeries)
		merged.CoachingInstitute = firstNonBlank(merged.CoachingInstitute, meta.CoachingInstitute)
		merged.TestCode = firstNonBlank(merged.TestCode, meta.TestCode)
		merged.TestDate = firstNonBlank(merged.TestDate, meta.TestDate)
		merged.Language = firstNonBlank(merged.Language, meta.Language)
		merged.Tags = cleanStringList(append(merged.Tags, meta.Tags...))
		merged.SearchHints = cleanStringList(append(merged.SearchHints, meta.SearchHints...))
	}
	return merged
}

func mergeDirectPDFChunkPages(results []directPDFChunkResult, pageCount int) ([]Page, []string) {
	selected := make(map[int]Page, pageCount)
	for _, result := range results {
		for _, page := range result.Response.Pages {
			current, found := selected[page.Number]
			if !found || directPDFPageScore(page) > directPDFPageScore(current) {
				selected[page.Number] = page
			}
		}
	}
	pages := make([]Page, 0, pageCount)
	warnings := []string{}
	for number := 1; number <= pageCount; number++ {
		page, found := selected[number]
		if !found {
			page = Page{
				Number:    number,
				Name:      fmt.Sprintf("page-%d", number),
				Kind:      "unknown",
				OCRIssues: []string{"No page-level assessment was returned for this page."},
			}
			warnings = append(warnings, fmt.Sprintf("Page %d has no page-level assessment and requires review.", number))
		}
		pages = append(pages, page)
	}
	return pages, warnings
}

func directPDFPageScore(page Page) int {
	score := len(strings.TrimSpace(page.Text))
	if page.Kind != "" && page.Kind != "unknown" {
		score += 1000
	}
	if page.OCRConfidence != nil {
		score += int(*page.OCRConfidence * 1000)
	}
	score += int(page.KindConfidence * 500)
	score -= page.UnclearCount * 10
	return score
}

func directPDFChunkUsage(results []directPDFChunkResult) (*provider.TokenUsage, int) {
	var usage *provider.TokenUsage
	calls := 0
	for _, result := range results {
		usage = addTokenUsage(usage, result.Response.Usage)
		calls += result.Response.APICalls
	}
	return usage, calls
}

func appendDirectPDFQualityWarnings(quality *AnalysisQuality, warnings []string) {
	if quality == nil {
		return
	}
	quality.Warnings = cleanStringList(append(quality.Warnings, warnings...))
	if len(warnings) > 0 {
		quality.RequiresReview = true
	}
}

func minIntValue(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
