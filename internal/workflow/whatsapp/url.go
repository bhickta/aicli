package whatsapp

import (
	"net/url"
	"strings"
)

func whatsappURL(recipient string, message string) string {
	query := url.Values{}
	query.Set("phone", recipient)
	query.Set("text", message)
	return "https://web.whatsapp.com/send?" + query.Encode()
}

func toolValue(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
