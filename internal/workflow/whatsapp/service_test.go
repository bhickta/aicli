package whatsapp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bhickta/aicli/internal/config"
)

type fakeRunner struct {
	starts []runnerCall
	calls  []runnerCall
}

type runnerCall struct {
	command string
	args    []string
}

func (f *fakeRunner) Start(_ context.Context, command string, args ...string) error {
	f.starts = append(f.starts, runnerCall{command: command, args: append([]string{}, args...)})
	return nil
}

func (f *fakeRunner) CombinedOutput(_ context.Context, command string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, runnerCall{command: command, args: append([]string{}, args...)})
	if len(args) > 0 && args[0] == "search" {
		return []byte("12345\n"), nil
	}
	return []byte{}, nil
}

func TestScheduleOpensSavedContactDraft(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	service := New(config.ToolConfig{Firefox: "firefox-test", XDoTool: "xdotool-test"}, runner)
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	resp, err := service.Schedule(context.Background(), ScheduleRequest{
		RecipientName:  "Mom",
		RecipientPhone: "+91 98765 43210",
		Message:        "hello",
		ScheduledAt:    "2026-05-20T12:00:00Z",
		AutoSend:       false,
	}, nil)
	if err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}
	if resp.RecipientName != "Mom" || resp.RecipientPhone != "919876543210" {
		t.Fatalf("response recipient = %#v, want normalized saved contact", resp)
	}
	if len(runner.starts) != 1 {
		t.Fatalf("starts = %d, want 1", len(runner.starts))
	}
	if runner.starts[0].command != "firefox-test" {
		t.Fatalf("firefox command = %q, want firefox-test", runner.starts[0].command)
	}
	if got := runner.starts[0].args[0]; !strings.Contains(got, "phone=919876543210") || !strings.Contains(got, "text=hello") {
		t.Fatalf("opened url = %q, want WhatsApp send URL", got)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("xdotool calls = %d, want none when auto_send=false", len(runner.calls))
	}
}

func TestNormalizeRequestRequiresPhoneNumber(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	_, err := normalizeRequest(ScheduleRequest{
		RecipientName: "Mom",
		Recipient:     "Mom",
		Message:       "hello",
		ScheduledAt:   "2026-05-20T12:00:00Z",
	}, now)
	if err == nil {
		t.Fatal("expected phone number validation error")
	}
}

func TestNormalizeRequestParsesLocalTimeAsIST(t *testing.T) {
	t.Parallel()

	ist := istLocation()
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, ist)
	req, err := normalizeRequest(ScheduleRequest{
		RecipientPhone: "+91 98765 43210",
		Message:        "hello",
		ScheduledAt:    "2026-05-20T18:30",
	}, now)
	if err != nil {
		t.Fatalf("normalizeRequest() error = %v", err)
	}
	if req.scheduledAt.Location().String() != ist.String() {
		t.Fatalf("location = %q, want %q", req.scheduledAt.Location(), ist)
	}
	if got := req.scheduledAt.Format(time.RFC3339); got != "2026-05-20T18:30:00+05:30" {
		t.Fatalf("scheduled_at = %q, want IST time", got)
	}
}

func TestActivateAndSendUsesActiveWindowEnter(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	service := New(config.ToolConfig{XDoTool: "xdotool-test"}, runner)
	service.sendFocusDelay = 0
	service.sendRetryDelay = 0

	attempts, err := service.activateAndSend(context.Background(), "12345", 2)
	if err != nil {
		t.Fatalf("activateAndSend() error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if len(runner.calls) != 3 {
		t.Fatalf("xdotool calls = %#v, want activate plus two key calls", runner.calls)
	}
	if got := strings.Join(runner.calls[0].args, " "); got != "windowactivate --sync 12345" {
		t.Fatalf("activate args = %q", got)
	}
	for _, call := range runner.calls[1:] {
		got := strings.Join(call.args, " ")
		if got != "key --clearmodifiers Return" {
			t.Fatalf("key args = %q, want active-window Return without --window", got)
		}
	}
}
