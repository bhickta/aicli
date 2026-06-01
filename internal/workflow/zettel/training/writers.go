package training

import (
	"encoding/json"
	"fmt"
	"os"
)

func writeJSONL(path string, records []exportRecord) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open training export %s: %w", path, err)
	}
	defer file.Close()
	for _, record := range records {
		data, err := json.Marshal(record.example)
		if err != nil {
			return fmt.Errorf("marshal training example: %w", err)
		}
		if _, err := file.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("write training example: %w", err)
		}
	}
	return nil
}

func writeShareGPTJSONL(path string, records []exportRecord) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open sharegpt training export %s: %w", path, err)
	}
	defer file.Close()
	for _, record := range records {
		data, err := json.Marshal(toShareGPT(record.example))
		if err != nil {
			return fmt.Errorf("marshal sharegpt training example: %w", err)
		}
		if _, err := file.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("write sharegpt training example: %w", err)
		}
	}
	return nil
}

func toShareGPT(example chatExample) shareGPTExample {
	conversations := make([]shareGPTMessage, 0, len(example.Messages))
	for _, message := range example.Messages {
		from := message.Role
		switch message.Role {
		case "user":
			from = "human"
		case "assistant":
			from = "gpt"
		}
		conversations = append(conversations, shareGPTMessage{
			From:  from,
			Value: message.Content,
		})
	}
	return shareGPTExample{Conversations: conversations}
}

func writeManifest(response TrainingExportResponse) error {
	data, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal training export manifest: %w", err)
	}
	return os.WriteFile(response.ManifestPath, append(data, '\n'), 0o600)
}
