package studyapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/storage"
	"github.com/bhickta/aicli/internal/workflow/analyze"
)

type studyMetadataResponse struct {
	Metadata  analyze.CopyMetadata    `json:"metadata"`
	Questions []studyMetadataQuestion `json:"questions"`
}

type studyMetadataQuestion struct {
	ID       string                   `json:"id"`
	Label    string                   `json:"label"`
	Metadata analyze.QuestionMetadata `json:"metadata"`
}

type studyMetadataPromptInput struct {
	Copy      studyMetadataCopyInput       `json:"copy"`
	Pages     []studyMetadataPageInput     `json:"pages"`
	Questions []studyMetadataQuestionInput `json:"questions"`
}

type studyMetadataCopyInput struct {
	ID            string `json:"id"`
	PDFName       string `json:"pdf_name"`
	SourcePath    string `json:"source_path"`
	CandidateName string `json:"candidate_name"`
	TestCode      string `json:"test_code"`
	Paper         string `json:"paper"`
	CopyDate      string `json:"copy_date"`
	PageCount     int    `json:"page_count"`
	QuestionCount int    `json:"question_count"`
}

type studyMetadataPageInput struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
}

type studyMetadataQuestionInput struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	PromptText  string `json:"prompt_text"`
	AnswerText  string `json:"answer_text"`
	SourcePages []int  `json:"source_pages"`
	Marks       int    `json:"marks,omitempty"`
	WordLimit   int    `json:"word_limit,omitempty"`
}

func (h *Handler) generateStudyBatchMetadata(
	ctx context.Context,
	store studyStore,
	copyRecord storage.StudyCopyRecord,
	llm provider.Provider,
	options studyBatchRunOptions,
) (studyBatchCopyResult, error) {
	pages, err := store.ListStudyPages(ctx, copyRecord.ID)
	if err != nil {
		return studyBatchCopyResult{}, fmt.Errorf("load study pages: %w", err)
	}
	questions, err := store.ListStudyQuestions(ctx, copyRecord.ID)
	if err != nil {
		return studyBatchCopyResult{}, fmt.Errorf("load study questions: %w", err)
	}
	if !options.ForceOCR && studyMetadataComplete(copyRecord, questions) {
		return studyBatchCopyResult{CopyID: copyRecord.ID, Status: "ready", CacheHit: true}, nil
	}
	input := studyMetadataInput(copyRecord, pages, questions)
	if !studyMetadataInputHasText(input) {
		return studyBatchCopyResult{}, fmt.Errorf("copy %s has no saved ocr/question text; run analysis first", copyRecord.ID)
	}
	content, usage, err := runStudyMetadataModel(ctx, llm, options, input)
	if err != nil {
		return studyBatchCopyResult{}, fmt.Errorf("generate metadata: %w", err)
	}
	payload, err := decodeStudyMetadataResponse(content)
	if err != nil {
		return studyBatchCopyResult{}, fmt.Errorf("parse metadata response: %w", err)
	}
	if err := saveStudyMetadata(ctx, store, copyRecord, questions, payload); err != nil {
		return studyBatchCopyResult{}, err
	}
	result := studyBatchCopyResult{CopyID: copyRecord.ID, Status: "ready", APICalls: 1}
	if usage != nil {
		result.InputTokens = usage.InputTokens
		result.OutputTokens = usage.OutputTokens
		result.TotalTokens = usage.TotalTokens
	}
	return result, nil
}

func runStudyMetadataModel(
	ctx context.Context,
	llm provider.Provider,
	options studyBatchRunOptions,
	input studyMetadataPromptInput,
) (string, *provider.TokenUsage, error) {
	if doc, ok := llm.(provider.DocumentProcessor); ok && strings.EqualFold(strings.TrimSpace(llm.ID()), "gemini") {
		response, err := doc.Document(ctx, provider.DocumentRequest{
			Model:       options.Model,
			Prompt:      studyMetadataInstruction() + "\n\nInput JSON is attached as text/plain.",
			Data:        []byte(studyMetadataInputJSON(input)),
			MIMEType:    "text/plain",
			Temperature: 0,
			MaxTokens:   6000,
		})
		if err != nil {
			return "", nil, err
		}
		return response.Content, response.Usage, nil
	}
	chat, err := llm.Chat(ctx, provider.ChatRequest{
		Model:       options.Model,
		Temperature: 0,
		MaxTokens:   6000,
		Messages: []provider.Message{{
			Role:    "user",
			Content: studyMetadataPrompt(input),
		}},
	})
	if err != nil {
		return "", nil, err
	}
	return chat.Content, chat.Usage, nil
}

func studyMetadataInput(
	copyRecord storage.StudyCopyRecord,
	pages []storage.StudyPageRecord,
	questions []storage.StudyQuestionRecord,
) studyMetadataPromptInput {
	input := studyMetadataPromptInput{
		Copy: studyMetadataCopyInput{
			ID:            copyRecord.ID,
			PDFName:       copyRecord.PDFName,
			SourcePath:    copyRecord.SourcePath,
			CandidateName: copyRecord.CandidateName,
			TestCode:      copyRecord.TestCode,
			Paper:         copyRecord.Paper,
			CopyDate:      copyRecord.CopyDate,
			PageCount:     copyRecord.PageCount,
			QuestionCount: copyRecord.QuestionCount,
		},
		Pages:     make([]studyMetadataPageInput, 0, len(pages)),
		Questions: make([]studyMetadataQuestionInput, 0, len(questions)),
	}
	for _, page := range pages {
		text := firstString(page.OCRText, page.RawOCR)
		if strings.TrimSpace(text) == "" {
			continue
		}
		input.Pages = append(input.Pages, studyMetadataPageInput{Number: page.PageNumber, Text: text})
	}
	for _, question := range questions {
		input.Questions = append(input.Questions, studyMetadataQuestionInput{
			ID:          question.ID,
			Label:       question.Label,
			PromptText:  question.PromptText,
			AnswerText:  question.AnswerText,
			SourcePages: question.SourcePages,
			Marks:       question.Marks,
			WordLimit:   question.WordLimit,
		})
	}
	return input
}

func studyMetadataPrompt(input studyMetadataPromptInput) string {
	return studyMetadataInstruction() + "\n\nInput JSON:\n" + studyMetadataInputJSON(input)
}

func studyMetadataInputJSON(input studyMetadataPromptInput) string {
	data, _ := json.MarshalIndent(input, "", "  ")
	return string(data)
}

func studyMetadataInstruction() string {
	return `Generate searchable metadata for this saved UPSC/Mains answer-copy.
Use only the provided saved OCR/question text. Do not run OCR. Do not split, merge, remove, rewrite, or summarize answers.
This copy can come from any coaching institute, test series, or layout. Treat institute branding as optional metadata only.

Return valid JSON only. No markdown fences, no prose, no trailing commas.
If a field is not visible or cannot be inferred confidently, return an empty string or empty array.
Return one questions[] object for every provided question id. Keep each id exactly unchanged.

Schema:
{
  "metadata": {
    "suggested_pdf_name": "clean searchable filename if enough evidence exists",
    "topper_name": "",
    "candidate_name": "",
    "rank": "",
    "exam": "",
    "year": "",
    "paper": "",
    "subject": "",
    "test_series": "",
    "coaching_institute": "",
    "test_code": "",
    "test_date": "",
    "language": "English / Hindi / mixed / other",
    "tags": ["short search tags"],
    "search_hints": ["alternate searchable names"],
    "notes": "short confidence note"
  },
  "questions": [
    {
      "id": "exact provided question id",
      "label": "same question label",
      "metadata": {
        "subject": "Polity / Economy / History / Geography / Ethics / Essay / Optional / other",
        "topic": "specific topic",
        "subtopic": "",
        "syllabus_area": "",
        "paper": "GS1 / GS2 / GS3 / GS4 / Essay / Optional / other",
        "question_type": "Discuss / Examine / Analyze / Critically evaluate / other",
        "demand": "main demand in one phrase",
        "difficulty": "easy / moderate / hard",
        "marks": 0,
        "word_limit": 0,
        "tags": ["short search tags"],
        "search_hints": ["alternate topic names"]
      }
    }
  ]
}`
}

func studyMetadataInputHasText(input studyMetadataPromptInput) bool {
	for _, page := range input.Pages {
		if strings.TrimSpace(page.Text) != "" {
			return true
		}
	}
	for _, question := range input.Questions {
		if strings.TrimSpace(question.PromptText)+strings.TrimSpace(question.AnswerText) != "" {
			return true
		}
	}
	return false
}

func decodeStudyMetadataResponse(content string) (studyMetadataResponse, error) {
	var payload studyMetadataResponse
	content = strings.TrimSpace(content)
	if err := json.Unmarshal([]byte(content), &payload); err == nil {
		return payload, nil
	}
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return studyMetadataResponse{}, fmt.Errorf("empty metadata json")
	}
	if err := json.Unmarshal([]byte(content[start:end+1]), &payload); err != nil {
		return studyMetadataResponse{}, err
	}
	return payload, nil
}

func saveStudyMetadata(
	ctx context.Context,
	store studyStore,
	copyRecord storage.StudyCopyRecord,
	questions []storage.StudyQuestionRecord,
	payload studyMetadataResponse,
) error {
	copyMeta := mergeCopyMetadata(existingCopyMetadata(copyRecord.MetadataJSON), cleanCopyMetadata(payload.Metadata))
	questionMeta := studyQuestionMetadataMap(payload.Questions)
	review := analyze.Response{
		PDFName:   copyRecord.PDFName,
		Metadata:  copyMeta,
		Questions: make([]analyze.Question, 0, len(questions)),
	}
	for _, question := range questions {
		meta := questionMeta[studyMetadataQuestionKey(question.ID)]
		if meta == nil {
			meta = questionMeta[studyMetadataQuestionKey(question.Label)]
		}
		if meta == nil {
			meta = existingQuestionMetadata(question.MetadataJSON)
		}
		if meta != nil {
			review.Questions = append(review.Questions, analyze.Question{
				ID:       question.ID,
				Label:    question.Label,
				Title:    question.PromptText,
				Metadata: meta,
			})
		}
	}
	record := storage.TopperReviewRecord{
		PDFName:    copyRecord.PDFName,
		SourcePath: copyRecord.SourcePath,
	}
	copyRecord.PDFName = studyCopyNameFromMetadata(copyRecord, record, review)
	copyRecord.CandidateName = firstString(copyRecord.CandidateName, copyCandidateName(copyMeta))
	copyRecord.TestCode = firstString(copyRecord.TestCode, copyTestCode(copyMeta))
	copyRecord.Paper = firstString(copyRecord.Paper, copyPaper(copyMeta))
	copyRecord.CopyDate = firstString(copyRecord.CopyDate, copyDate(copyMeta))
	copyRecord.MetadataJSON = firstString(studyCopyMetadataJSON(review), copyRecord.MetadataJSON)
	if err := store.SaveStudyCopy(ctx, copyRecord); err != nil {
		return fmt.Errorf("save study copy metadata: %w", err)
	}
	for _, question := range questions {
		meta := questionMeta[studyMetadataQuestionKey(question.ID)]
		if meta == nil {
			meta = questionMeta[studyMetadataQuestionKey(question.Label)]
		}
		if meta == nil {
			continue
		}
		question.MetadataJSON = jsonString(meta)
		question.Marks = firstInt(meta.Marks, question.Marks)
		question.WordLimit = firstInt(meta.WordLimit, question.WordLimit)
		if err := store.SaveStudyQuestion(ctx, question); err != nil {
			return fmt.Errorf("save question metadata: %w", err)
		}
	}
	return nil
}

func studyMetadataComplete(copyRecord storage.StudyCopyRecord, questions []storage.StudyQuestionRecord) bool {
	if strings.TrimSpace(copyRecord.MetadataJSON) == "" {
		return false
	}
	for _, question := range questions {
		if strings.TrimSpace(question.MetadataJSON) == "" {
			return false
		}
	}
	return true
}

func studyQuestionMetadataMap(items []studyMetadataQuestion) map[string]*analyze.QuestionMetadata {
	out := make(map[string]*analyze.QuestionMetadata, len(items)*2)
	for _, item := range items {
		meta := cleanQuestionMetadata(item.Metadata)
		if meta == nil {
			continue
		}
		if key := studyMetadataQuestionKey(item.ID); key != "" {
			out[key] = meta
		}
		if key := studyMetadataQuestionKey(item.Label); key != "" {
			out[key] = meta
		}
	}
	return out
}

func studyMetadataQuestionKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func existingCopyMetadata(raw string) *analyze.CopyMetadata {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var wrapped struct {
		Copy analyze.CopyMetadata `json:"copy"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapped); err == nil {
		if meta := cleanCopyMetadata(wrapped.Copy); meta != nil {
			return meta
		}
	}
	var meta analyze.CopyMetadata
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return nil
	}
	return cleanCopyMetadata(meta)
}

func existingQuestionMetadata(raw string) *analyze.QuestionMetadata {
	var meta analyze.QuestionMetadata
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return nil
	}
	return cleanQuestionMetadata(meta)
}

func mergeCopyMetadata(oldMeta *analyze.CopyMetadata, newMeta *analyze.CopyMetadata) *analyze.CopyMetadata {
	if oldMeta == nil {
		return newMeta
	}
	if newMeta == nil {
		return oldMeta
	}
	merged := *newMeta
	merged.SuggestedPDFName = firstString(merged.SuggestedPDFName, oldMeta.SuggestedPDFName)
	merged.TopperName = firstString(merged.TopperName, oldMeta.TopperName)
	merged.CandidateName = firstString(merged.CandidateName, oldMeta.CandidateName)
	merged.Rank = firstString(merged.Rank, oldMeta.Rank)
	merged.Exam = firstString(merged.Exam, oldMeta.Exam)
	merged.Year = firstString(merged.Year, oldMeta.Year)
	merged.Paper = firstString(merged.Paper, oldMeta.Paper)
	merged.Subject = firstString(merged.Subject, oldMeta.Subject)
	merged.TestSeries = firstString(merged.TestSeries, oldMeta.TestSeries)
	merged.CoachingInstitute = firstString(merged.CoachingInstitute, oldMeta.CoachingInstitute)
	merged.TestCode = firstString(merged.TestCode, oldMeta.TestCode)
	merged.TestDate = firstString(merged.TestDate, oldMeta.TestDate)
	merged.Language = firstString(merged.Language, oldMeta.Language)
	if len(merged.Tags) == 0 {
		merged.Tags = oldMeta.Tags
	}
	if len(merged.SearchHints) == 0 {
		merged.SearchHints = oldMeta.SearchHints
	}
	merged.Notes = firstString(merged.Notes, oldMeta.Notes)
	return &merged
}

func cleanCopyMetadata(meta analyze.CopyMetadata) *analyze.CopyMetadata {
	meta = analyze.CopyMetadata{
		SuggestedPDFName:  strings.TrimSpace(meta.SuggestedPDFName),
		TopperName:        strings.TrimSpace(meta.TopperName),
		CandidateName:     strings.TrimSpace(meta.CandidateName),
		Rank:              strings.TrimSpace(meta.Rank),
		Exam:              strings.TrimSpace(meta.Exam),
		Year:              strings.TrimSpace(meta.Year),
		Paper:             strings.TrimSpace(meta.Paper),
		Subject:           strings.TrimSpace(meta.Subject),
		TestSeries:        strings.TrimSpace(meta.TestSeries),
		CoachingInstitute: strings.TrimSpace(meta.CoachingInstitute),
		TestCode:          strings.TrimSpace(meta.TestCode),
		TestDate:          strings.TrimSpace(meta.TestDate),
		Language:          strings.TrimSpace(meta.Language),
		Tags:              cleanStudyMetadataList(meta.Tags),
		SearchHints:       cleanStudyMetadataList(meta.SearchHints),
		Notes:             strings.TrimSpace(meta.Notes),
	}
	if meta.SuggestedPDFName+meta.TopperName+meta.CandidateName+meta.Rank+meta.Exam+
		meta.Year+meta.Paper+meta.Subject+meta.TestSeries+meta.CoachingInstitute+
		meta.TestCode+meta.TestDate+meta.Language+strings.Join(meta.Tags, "")+
		strings.Join(meta.SearchHints, "")+meta.Notes == "" {
		return nil
	}
	return &meta
}

func cleanQuestionMetadata(meta analyze.QuestionMetadata) *analyze.QuestionMetadata {
	meta = analyze.QuestionMetadata{
		Subject:      strings.TrimSpace(meta.Subject),
		Topic:        strings.TrimSpace(meta.Topic),
		Subtopic:     strings.TrimSpace(meta.Subtopic),
		SyllabusArea: strings.TrimSpace(meta.SyllabusArea),
		Paper:        strings.TrimSpace(meta.Paper),
		QuestionType: strings.TrimSpace(meta.QuestionType),
		Demand:       strings.TrimSpace(meta.Demand),
		Difficulty:   strings.TrimSpace(meta.Difficulty),
		Marks:        nonNegativeStudyInt(meta.Marks),
		WordLimit:    nonNegativeStudyInt(meta.WordLimit),
		Tags:         cleanStudyMetadataList(meta.Tags),
		SearchHints:  cleanStudyMetadataList(meta.SearchHints),
	}
	if meta.Subject+meta.Topic+meta.Subtopic+meta.SyllabusArea+meta.Paper+
		meta.QuestionType+meta.Demand+meta.Difficulty+strings.Join(meta.Tags, "")+
		strings.Join(meta.SearchHints, "") == "" && meta.Marks == 0 && meta.WordLimit == 0 {
		return nil
	}
	return &meta
}

func cleanStudyMetadataList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

func nonNegativeStudyInt(value int) int {
	if value < 0 {
		return 0
	}
	return value
}
