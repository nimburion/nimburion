package scheduler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	MisfirePolicySkip     = "skip"
	MisfirePolicyFireOnce = "fire_once"
)

// Task describes one scheduler entry that dispatches a job.
type Task struct {
	Name           string
	Schedule       string
	Queue          string
	JobName        string
	Payload        []byte
	Headers        map[string]string
	TenantID       string
	IdempotencyKey string
	Timezone       string
	LockTTL        time.Duration
	MisfirePolicy  string
}

func (t *Task) normalize() {
	if strings.TrimSpace(t.MisfirePolicy) == "" {
		t.MisfirePolicy = MisfirePolicySkip
	}
}

// Validate verifies required fields and schedule syntax.
func (t *Task) Validate() error {
	if t == nil {
		return errors.New("task is nil")
	}
	t.normalize()

	if strings.TrimSpace(t.Name) == "" {
		return errors.New("task name is required")
	}
	if strings.TrimSpace(t.Schedule) == "" {
		return errors.New("task schedule is required")
	}
	if strings.TrimSpace(t.Queue) == "" {
		return errors.New("task queue is required")
	}
	if strings.TrimSpace(t.JobName) == "" {
		return errors.New("task job_name is required")
	}
	if t.MisfirePolicy != MisfirePolicySkip && t.MisfirePolicy != MisfirePolicyFireOnce {
		return fmt.Errorf("invalid task misfire policy %q", t.MisfirePolicy)
	}
	if _, err := t.nextRun(time.Now().UTC()); err != nil {
		return err
	}
	return nil
}

func (t *Task) location() (*time.Location, error) {
	if strings.TrimSpace(t.Timezone) == "" {
		return time.UTC, nil
	}
	loc, err := time.LoadLocation(strings.TrimSpace(t.Timezone))
	if err != nil {
		return nil, fmt.Errorf("invalid task timezone: %w", err)
	}
	return loc, nil
}

func (t *Task) nextRun(now time.Time) (time.Time, error) {
	loc, err := t.location()
	if err != nil {
		return time.Time{}, err
	}
	return nextRunForSchedule(strings.TrimSpace(t.Schedule), now.In(loc), loc)
}

func nextRunForSchedule(schedule string, now time.Time, loc *time.Location) (time.Time, error) {
	if strings.HasPrefix(schedule, "@every ") {
		durationRaw := strings.TrimSpace(strings.TrimPrefix(schedule, "@every "))
		interval, err := time.ParseDuration(durationRaw)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid @every duration: %w", err)
		}
		if interval <= 0 {
			return time.Time{}, errors.New("@every duration must be > 0")
		}
		return now.Add(interval).UTC(), nil
	}

	fields := strings.Fields(schedule)
	if len(fields) != 5 {
		return time.Time{}, fmt.Errorf("unsupported schedule format %q", schedule)
	}
	if fields[1] != "*" || fields[2] != "*" || fields[3] != "*" || fields[4] != "*" {
		return time.Time{}, fmt.Errorf("unsupported schedule %q: only minute-based schedules are supported", schedule)
	}

	minuteField := fields[0]
	base := now.Truncate(time.Minute).Add(time.Minute)
	switch {
	case minuteField == "*":
		return base.UTC(), nil
	case strings.HasPrefix(minuteField, "*/"):
		stepRaw := strings.TrimSpace(strings.TrimPrefix(minuteField, "*/"))
		step, err := strconv.Atoi(stepRaw)
		if err != nil || step <= 0 {
			return time.Time{}, fmt.Errorf("invalid minute step %q", minuteField)
		}
		candidate := base
		for candidate.Minute()%step != 0 {
			candidate = candidate.Add(time.Minute)
		}
		return candidate.In(loc).UTC(), nil
	default:
		minute, err := strconv.Atoi(minuteField)
		if err != nil || minute < 0 || minute > 59 {
			return time.Time{}, fmt.Errorf("invalid minute value %q", minuteField)
		}
		candidate := time.Date(base.Year(), base.Month(), base.Day(), base.Hour(), minute, 0, 0, loc)
		if !candidate.After(now) {
			candidate = candidate.Add(time.Hour)
		}
		return candidate.UTC(), nil
	}
}
