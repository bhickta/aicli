package analyze

import (
	"encoding/json"
	"fmt"
	"math"
)

type questionScorecardJSON struct {
	DemandFulfilment    json.Number `json:"demand_fulfilment"`
	Structure           json.Number `json:"structure"`
	ContentDepth        json.Number `json:"content_depth"`
	Evidence            json.Number `json:"evidence"`
	Multidimensionality json.Number `json:"multidimensionality"`
	Presentation        json.Number `json:"presentation"`
	Conclusion          json.Number `json:"conclusion"`
	OverallPercent      json.Number `json:"overall_percent"`
	EstimatedBand       string      `json:"estimated_band"`
	Confidence          string      `json:"confidence"`
	Rationale           string      `json:"rationale"`
}

func (s *QuestionScorecard) UnmarshalJSON(data []byte) error {
	var payload questionScorecardJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	var err error
	if s.DemandFulfilment, err = roundedJSONScore(payload.DemandFulfilment, 0, 5); err != nil {
		return fmt.Errorf("demand_fulfilment: %w", err)
	}
	if s.Structure, err = roundedJSONScore(payload.Structure, 0, 5); err != nil {
		return fmt.Errorf("structure: %w", err)
	}
	if s.ContentDepth, err = roundedJSONScore(payload.ContentDepth, 0, 5); err != nil {
		return fmt.Errorf("content_depth: %w", err)
	}
	if s.Evidence, err = roundedJSONScore(payload.Evidence, 0, 5); err != nil {
		return fmt.Errorf("evidence: %w", err)
	}
	if s.Multidimensionality, err = roundedJSONScore(payload.Multidimensionality, 0, 5); err != nil {
		return fmt.Errorf("multidimensionality: %w", err)
	}
	if s.Presentation, err = roundedJSONScore(payload.Presentation, 0, 5); err != nil {
		return fmt.Errorf("presentation: %w", err)
	}
	if s.Conclusion, err = roundedJSONScore(payload.Conclusion, 0, 5); err != nil {
		return fmt.Errorf("conclusion: %w", err)
	}
	if s.OverallPercent, err = roundedJSONScore(payload.OverallPercent, 0, 100); err != nil {
		return fmt.Errorf("overall_percent: %w", err)
	}
	s.EstimatedBand = payload.EstimatedBand
	s.Confidence = payload.Confidence
	s.Rationale = payload.Rationale
	return nil
}

func roundedJSONScore(value json.Number, minimum int, maximum int) (int, error) {
	if value == "" {
		return 0, nil
	}
	number, err := value.Float64()
	if err != nil {
		return 0, err
	}
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, fmt.Errorf("score %q is not finite", value)
	}
	if number <= float64(minimum) {
		return minimum, nil
	}
	if number >= float64(maximum) {
		return maximum, nil
	}
	return int(math.Round(number)), nil
}
