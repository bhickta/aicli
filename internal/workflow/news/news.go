package news

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/xuri/excelize/v2"
)

type Item struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Source  string `json:"source"`
	Date    string `json:"date"`
}

type Request struct {
	Model      string `json:"model"`
	Path       string `json:"path"`
	OutputPath string `json:"output_path"`
	UseLLM     bool   `json:"use_llm"`
	ProviderID string `json:"provider_id"`
}

type Response struct {
	Items       []Item    `json:"items"`
	Clusters    []Cluster `json:"clusters"`
	Duplicates  int       `json:"duplicates"`
	LLMSummary  string    `json:"llm_summary,omitempty"`
	SourceCount int       `json:"source_count"`
	OutputPath  string    `json:"output_path,omitempty"`
}

type Cluster struct {
	Items []Item  `json:"items"`
	Score float64 `json:"score"`
}

type Service struct {
	provider provider.Provider
}

func New(provider provider.Provider) *Service {
	return &Service{provider: provider}
}

func (s *Service) Run(ctx context.Context, req Request) (Response, error) {
	if strings.TrimSpace(req.Path) == "" {
		return Response{}, errors.New("path is required")
	}
	items, err := loadItems(req.Path)
	if err != nil {
		return Response{}, err
	}
	deduped, duplicates := dedupe(items)
	res := Response{Items: deduped, Clusters: cluster(deduped, 0.35), Duplicates: duplicates, SourceCount: len(items)}
	if req.OutputPath != "" {
		if err := exportXLSX(req.OutputPath, deduped); err != nil {
			return Response{}, err
		}
		res.OutputPath = req.OutputPath
	}
	if req.UseLLM && s.provider != nil && len(deduped) > 0 {
		payload, _ := json.Marshal(deduped)
		chat, err := s.provider.Chat(ctx, provider.ChatRequest{
			Model: req.Model,
			Messages: []provider.Message{
				{Role: "user", Content: "Merge and summarize these deduplicated news items. Output concise Markdown.\n\n" + string(payload)},
			},
			Temperature: 0,
			MaxTokens:   2000,
		})
		if err != nil {
			return Response{}, err
		}
		res.LLMSummary = strings.TrimSpace(chat.Content)
	}
	return res, nil
}

func loadItems(path string) ([]Item, error) {
	if strings.EqualFold(filepath.Ext(path), ".xlsx") {
		return parseXLSX(path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseItems(data)
}

func parseItems(data []byte) ([]Item, error) {
	var items []Item
	if err := json.Unmarshal(data, &items); err == nil {
		return items, nil
	}
	var wrapped struct {
		Items []Item `json:"items"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return nil, err
	}
	return wrapped.Items, nil
}

func parseXLSX(path string) ([]Item, error) {
	file, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	sheets := file.GetSheetList()
	if len(sheets) == 0 {
		return []Item{}, nil
	}
	rows, err := file.GetRows(sheets[0])
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []Item{}, nil
	}
	header := map[string]int{}
	for i, cell := range rows[0] {
		header[normalize(cell)] = i
	}
	items := []Item{}
	for _, row := range rows[1:] {
		items = append(items, Item{
			Title:   cell(row, header["title"]),
			Content: cell(row, header["content"]),
			Source:  cell(row, header["source"]),
			Date:    cell(row, header["date"]),
		})
	}
	return items, nil
}

func exportXLSX(path string, items []Item) error {
	file := excelize.NewFile()
	sheet := file.GetSheetName(0)
	headers := []string{"title", "content", "source", "date"}
	for i, header := range headers {
		cellName, _ := excelize.CoordinatesToCellName(i+1, 1)
		file.SetCellValue(sheet, cellName, header)
	}
	for rowIndex, item := range items {
		values := []string{item.Title, item.Content, item.Source, item.Date}
		for colIndex, value := range values {
			cellName, _ := excelize.CoordinatesToCellName(colIndex+1, rowIndex+2)
			file.SetCellValue(sheet, cellName, value)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	return file.SaveAs(path)
}

func cell(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return row[index]
}

func dedupe(items []Item) ([]Item, int) {
	seen := map[string]Item{}
	for _, item := range items {
		key := normalize(item.Title)
		if key == "" {
			key = normalize(item.Content)
		}
		if key == "" {
			continue
		}
		if existing, ok := seen[key]; ok {
			if len(item.Content) > len(existing.Content) {
				seen[key] = item
			}
			continue
		}
		seen[key] = item
	}
	out := make([]Item, 0, len(seen))
	for _, item := range seen {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Title < out[j].Title
	})
	return out, len(items) - len(out)
}

func normalize(value string) string {
	fields := strings.Fields(strings.ToLower(value))
	return strings.Join(fields, " ")
}

func cluster(items []Item, threshold float64) []Cluster {
	clusters := []Cluster{}
	used := make([]bool, len(items))
	for i, item := range items {
		if used[i] {
			continue
		}
		current := Cluster{Items: []Item{item}, Score: 1}
		used[i] = true
		for j := i + 1; j < len(items); j++ {
			if used[j] {
				continue
			}
			score := similarity(item, items[j])
			if score >= threshold {
				current.Items = append(current.Items, items[j])
				if score > current.Score {
					current.Score = score
				}
				used[j] = true
			}
		}
		clusters = append(clusters, current)
	}
	return clusters
}

func similarity(a Item, b Item) float64 {
	left := tokenSet(a.Title + " " + a.Content)
	right := tokenSet(b.Title + " " + b.Content)
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	intersection := 0
	for token := range left {
		if right[token] {
			intersection++
		}
	}
	union := len(left) + len(right) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func tokenSet(value string) map[string]bool {
	words := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
	out := map[string]bool{}
	for _, word := range words {
		if len(word) < 3 {
			continue
		}
		out[word] = true
	}
	return out
}
