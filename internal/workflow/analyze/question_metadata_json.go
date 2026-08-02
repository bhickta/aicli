package analyze

import (
	"encoding/json"
	"fmt"
	"math"
)

const maxQuestionMarks = 1000.0

type questionMetadataJSON QuestionMetadata

func (m *QuestionMetadata) UnmarshalJSON(data []byte) error {
	var payload questionMetadataJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	if math.IsNaN(payload.Marks) || math.IsInf(payload.Marks, 0) || payload.Marks < 0 || payload.Marks > maxQuestionMarks {
		return fmt.Errorf("marks must be a finite JSON number between 0 and %g", maxQuestionMarks)
	}
	*m = QuestionMetadata(payload)
	return nil
}
