package training

func systemContent(example chatExample) string {
	for _, message := range example.Messages {
		if message.Role == "system" {
			return message.Content
		}
	}
	return ""
}

func userContent(example chatExample) string {
	for _, message := range example.Messages {
		if message.Role == "user" {
			return message.Content
		}
	}
	return ""
}

func assistantContent(example chatExample) string {
	for i := len(example.Messages) - 1; i >= 0; i-- {
		if example.Messages[i].Role == "assistant" {
			return example.Messages[i].Content
		}
	}
	return ""
}
