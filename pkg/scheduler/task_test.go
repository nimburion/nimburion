package scheduler

import (
	"testing"
	"time"
)

func TestTaskValidate(t *testing.T) {
	task := &Task{
		Name:     "billing-close-day",
		Schedule: "@every 30s",
		Queue:    "billing",
		JobName:  "billing.close_day",
		Payload:  []byte(`{"source":"scheduler"}`),
	}
	if err := task.Validate(); err != nil {
		t.Fatalf("expected valid task, got %v", err)
	}
}

func TestNextRunForSchedule_Every(t *testing.T) {
	now := time.Date(2026, 2, 26, 12, 0, 0, 0, time.UTC)
	next, err := nextRunForSchedule("@every 2s", now, time.UTC)
	if err != nil {
		t.Fatalf("nextRunForSchedule error: %v", err)
	}
	expected := now.Add(2 * time.Second)
	if !next.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, next)
	}
}

func TestNextRunForSchedule_MinuteStep(t *testing.T) {
	now := time.Date(2026, 2, 26, 12, 3, 40, 0, time.UTC)
	next, err := nextRunForSchedule("*/5 * * * *", now, time.UTC)
	if err != nil {
		t.Fatalf("nextRunForSchedule error: %v", err)
	}
	expected := time.Date(2026, 2, 26, 12, 5, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, next)
	}
}

func TestNextRunForSchedule_FixedMinute(t *testing.T) {
	now := time.Date(2026, 2, 26, 12, 35, 0, 0, time.UTC)
	next, err := nextRunForSchedule("15 * * * *", now, time.UTC)
	if err != nil {
		t.Fatalf("nextRunForSchedule error: %v", err)
	}
	expected := time.Date(2026, 2, 26, 13, 15, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, next)
	}
}
