package jobs

import (
	"strings"
	"testing"
	"time"
)

func validJob() *Job {
	return &Job{
		ID:             "job-1",
		Name:           "billing.daily-close",
		Queue:          "maintenance",
		Payload:        []byte(`{"tenant":"tenant-a"}`),
		Headers:        map[string]string{"source": "test"},
		ContentType:    "application/json",
		TenantID:       "tenant-a",
		CorrelationID:  "corr-1",
		IdempotencyKey: "billing.daily-close:2026-02-26",
		RunAt:          time.Date(2026, 2, 26, 2, 0, 0, 0, time.UTC),
		Attempt:        1,
		MaxAttempts:    5,
		CreatedAt:      time.Date(2026, 2, 26, 1, 59, 0, 0, time.UTC),
	}
}

func TestJobValidate(t *testing.T) {
	job := validJob()
	if err := job.Validate(); err != nil {
		t.Fatalf("expected valid job, got %v", err)
	}
}

func TestJobValidateMissingFields(t *testing.T) {
	job := validJob()
	job.ID = " "
	if err := job.Validate(); err == nil || !strings.Contains(err.Error(), "id") {
		t.Fatalf("expected id validation error, got %v", err)
	}

	job = validJob()
	job.Name = ""
	if err := job.Validate(); err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("expected name validation error, got %v", err)
	}

	job = validJob()
	job.Queue = ""
	if err := job.Validate(); err == nil || !strings.Contains(err.Error(), "queue") {
		t.Fatalf("expected queue validation error, got %v", err)
	}

	job = validJob()
	job.Payload = nil
	if err := job.Validate(); err == nil || !strings.Contains(err.Error(), "payload") {
		t.Fatalf("expected payload validation error, got %v", err)
	}
}

func TestJobToAndFromEventBusMessage(t *testing.T) {
	job := validJob()
	msg, err := job.ToEventBusMessage()
	if err != nil {
		t.Fatalf("to message failed: %v", err)
	}

	if msg.ID != job.ID {
		t.Fatalf("message id mismatch: got %q want %q", msg.ID, job.ID)
	}
	if msg.ContentType != job.ContentType {
		t.Fatalf("content type mismatch: got %q want %q", msg.ContentType, job.ContentType)
	}
	if msg.Headers[HeaderJobName] != job.Name {
		t.Fatalf("missing job_name header")
	}
	if msg.Headers["tenant_id"] != job.TenantID {
		t.Fatalf("expected tenant_id compatibility header")
	}

	decoded, err := JobFromEventBusMessage(msg)
	if err != nil {
		t.Fatalf("from message failed: %v", err)
	}

	if decoded.Name != job.Name {
		t.Fatalf("name mismatch: got %q want %q", decoded.Name, job.Name)
	}
	if decoded.Queue != job.Queue {
		t.Fatalf("queue mismatch: got %q want %q", decoded.Queue, job.Queue)
	}
	if string(decoded.Payload) != string(job.Payload) {
		t.Fatalf("payload mismatch: got %s want %s", string(decoded.Payload), string(job.Payload))
	}
	if decoded.Attempt != job.Attempt {
		t.Fatalf("attempt mismatch: got %d want %d", decoded.Attempt, job.Attempt)
	}
	if decoded.MaxAttempts != job.MaxAttempts {
		t.Fatalf("max attempts mismatch: got %d want %d", decoded.MaxAttempts, job.MaxAttempts)
	}
}

func TestJobFromEventBusMessageWithQueueFallback(t *testing.T) {
	job := validJob()
	msg, err := job.ToEventBusMessage()
	if err != nil {
		t.Fatalf("to message failed: %v", err)
	}

	delete(msg.Headers, HeaderJobQueue)
	decoded, err := JobFromEventBusMessageWithQueue(msg, "fallback-queue")
	if err != nil {
		t.Fatalf("from message failed: %v", err)
	}
	if decoded.Queue != "fallback-queue" {
		t.Fatalf("expected fallback queue, got %q", decoded.Queue)
	}
}

func TestJobFromEventBusMessageInvalidHeaders(t *testing.T) {
	job := validJob()
	msg, err := job.ToEventBusMessage()
	if err != nil {
		t.Fatalf("to message failed: %v", err)
	}

	msg.Headers[HeaderJobAttempt] = "invalid"
	if _, err := JobFromEventBusMessage(msg); err == nil {
		t.Fatal("expected attempt parse error")
	}
}

func TestJobPartitionKey(t *testing.T) {
	if got, want := JobPartitionKey("tenant-a", "billing.close", "job-1"), "tenant-a:billing.close"; got != want {
		t.Fatalf("unexpected partition key: got %q want %q", got, want)
	}
	if got, want := JobPartitionKey("", "billing.close", "job-1"), "billing.close:job-1"; got != want {
		t.Fatalf("unexpected partition key fallback: got %q want %q", got, want)
	}
	if got, want := JobPartitionKey("", "", "job-1"), "job-1"; got != want {
		t.Fatalf("unexpected partition key fallback: got %q want %q", got, want)
	}
}

func TestMarshalPayloadJSON(t *testing.T) {
	payload, err := MarshalPayloadJSON(map[string]any{"id": "job-1"})
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if len(payload) == 0 {
		t.Fatal("expected non-empty payload")
	}
}
