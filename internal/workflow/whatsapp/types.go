package whatsapp

type ScheduleRequest struct {
	Recipient      string `json:"recipient"`
	RecipientName  string `json:"recipient_name"`
	RecipientPhone string `json:"recipient_phone"`
	Message        string `json:"message"`
	ScheduledAt    string `json:"scheduled_at"`
	AutoSend       bool   `json:"auto_send"`
	WaitSeconds    int    `json:"wait_seconds"`
	SendRetries    int    `json:"send_retries"`
}

type ScheduleResponse struct {
	RecipientName  string `json:"recipient_name"`
	RecipientPhone string `json:"recipient_phone"`
	ScheduledAt    string `json:"scheduled_at"`
	AutoSend       bool   `json:"auto_send"`
	URL            string `json:"url"`
	Output         string `json:"output"`
	SendAttempts   int    `json:"send_attempts,omitempty"`
}

type ProgressFunc = func(stage string, currentStep, totalSteps int)
