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
	Result       directPDFChunkResult `json:"result"`
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
		cached, found, err := loadDirectPDFChunkCheckpoint(
			checkpointDir,
			chunk,
			sourceSHA256,
			model,
		)
		if err != nil {
			return Response{}, err
		}
		if found {
			results = append(results, cached)
			s.logInfo("direct PDF chunk checkpoint reused", "chunk", chunk.Index+1, "first_page", chunk.FirstPage, "last_page", chunk.LastPage)
			continue
		}

		result, err := s.extractDirectPDFChunk(ctx, req, pdfName, pageCount, chunk, tempDir, processor)
		if err != nil {
			return Response{}, fmt.Errorf("extract direct PDF chunk %d pages %d-%d: %w", chunk.Index+1, chunk.FirstPage, chunk.LastPage, err)
		}
		if err := saveDirectPDFChunkCheckpoint(
			checkpointDir,
			directPDFChunkCheckpoint{
				Version:      directPDFCheckpointVersion,
				SourceSHA256: sourceSHA256,
				Model:        model,
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
		results,
	)
	if err != nil {
		return Response{}, err
	}
	usage, calls := directPDFChunkUsage(results)
	usage = addTokenUsage(usage, reconcileUsage)
	calls += reconcileCalls
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

func (s *Service) extractDirectPDFChunk(
	ctx context.Context,
	req Request,
	pdfName string,
	pageCount int,
	chunk directPDFChunk,
	tempDir string,
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
	prompts := []string{
		directPDFChunkPrompt(pdfName, pageCount, chunk, false),
		directPDFChunkPrompt(pdfName, pageCount, chunk, true),
	}
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
			return directPDFChunkResult{}, err
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
			return directPDFChunkResult{}, err
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

func directPDFChunkPrompt(pdfName string, pageCount int, chunk directPDFChunk, strict bool) string {
	localPages := chunk.LastPage - chunk.FirstPage + 1
	prefix := fmt.Sprintf(`The attached PDF is chunk %d of a larger %d-page answer-copy.
It contains original global pages %d-%d, represented inside this attachment as local pages 1-%d.
In every pages[].number and questions[].source_pages value, use attachment-local page numbers only. The application will apply the global offset.
Return a pages[] entry for every attached page, including cover, question-paper, evaluation, and blank pages.
Analyze only visible content in this attachment. A question that begins or ends at a chunk edge may be incomplete; preserve its visible text and source pages without inventing missing content.

`, chunk.Index+1, pageCount, chunk.FirstPage, chunk.LastPage, localPages)
	return prefix + oneShotPDFPromptBody(pdfName, strict)
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
		checkpoint.Result.Chunk != chunk {
		return directPDFChunkResult{}, false, fmt.Errorf("direct PDF checkpoint %s does not match the requested source, model, or page range", path)
	}
	return checkpoint.Result, true, nil
}

func saveDirectPDFChunkCheckpoint(dir string, checkpoint directPDFChunkCheckpoint) error {
	path := directPDFChunkCheckpointPath(dir, checkpoint.Result.Chunk)
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		_, found, loadErr := loadDirectPDFChunkCheckpoint(
			dir,
			checkpoint.Result.Chunk,
			checkpoint.SourceSHA256,
			checkpoint.Model,
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
	if err := os.Link(tempPath, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return err
	}
	return nil
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
		merged.Notes = firstNonBlank(merged.Notes, meta.Notes)
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
