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

	maxCronSearchIterations = 5 * 366 * 24 * 60
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

	cronExpr, err := parseCronExpression(fields)
	if err != nil {
		return time.Time{}, err
	}

	candidate := now.Truncate(time.Minute).Add(time.Minute)
	for iteration := 0; iteration < maxCronSearchIterations; iteration++ {
		localCandidate := candidate.In(loc)
		if cronExpr.matches(localCandidate) {
			return localCandidate.UTC(), nil
		}
		candidate = candidate.Add(time.Minute)
	}

	return time.Time{}, fmt.Errorf("unable to find next run for schedule %q", schedule)
}

type cronFieldMatcher struct {
	any    bool
	values map[int]struct{}
}

func (m cronFieldMatcher) contains(value int) bool {
	if m.any {
		return true
	}
	_, ok := m.values[value]
	return ok
}

type cronExpression struct {
	minute     cronFieldMatcher
	hour       cronFieldMatcher
	dayOfMonth cronFieldMatcher
	month      cronFieldMatcher
	dayOfWeek  cronFieldMatcher
}

func (e cronExpression) matches(candidate time.Time) bool {
	if !e.minute.contains(candidate.Minute()) {
		return false
	}
	if !e.hour.contains(candidate.Hour()) {
		return false
	}
	if !e.month.contains(int(candidate.Month())) {
		return false
	}

	dayOfMonthMatch := e.dayOfMonth.contains(candidate.Day())
	dayOfWeekValue := int(candidate.Weekday())
	dayOfWeekMatch := e.dayOfWeek.contains(dayOfWeekValue)
	dayMatches := false
	switch {
	case e.dayOfMonth.any && e.dayOfWeek.any:
		dayMatches = true
	case e.dayOfMonth.any:
		dayMatches = dayOfWeekMatch
	case e.dayOfWeek.any:
		dayMatches = dayOfMonthMatch
	default:
		dayMatches = dayOfMonthMatch || dayOfWeekMatch
	}

	return dayMatches
}

func parseCronExpression(fields []string) (*cronExpression, error) {
	minute, err := parseCronField(fields[0], 0, 59, false)
	if err != nil {
		return nil, fmt.Errorf("invalid minute field %q: %w", fields[0], err)
	}
	hour, err := parseCronField(fields[1], 0, 23, false)
	if err != nil {
		return nil, fmt.Errorf("invalid hour field %q: %w", fields[1], err)
	}
	dayOfMonth, err := parseCronField(fields[2], 1, 31, false)
	if err != nil {
		return nil, fmt.Errorf("invalid day-of-month field %q: %w", fields[2], err)
	}
	month, err := parseCronField(fields[3], 1, 12, false)
	if err != nil {
		return nil, fmt.Errorf("invalid month field %q: %w", fields[3], err)
	}
	dayOfWeek, err := parseCronField(fields[4], 0, 7, true)
	if err != nil {
		return nil, fmt.Errorf("invalid day-of-week field %q: %w", fields[4], err)
	}

	return &cronExpression{
		minute:     minute,
		hour:       hour,
		dayOfMonth: dayOfMonth,
		month:      month,
		dayOfWeek:  dayOfWeek,
	}, nil
}

func parseCronField(raw string, minValue, maxValue int, normalizeSunday bool) (cronFieldMatcher, error) {
	field := strings.TrimSpace(raw)
	if field == "" {
		return cronFieldMatcher{}, errors.New("empty field")
	}
	if field == "*" {
		return cronFieldMatcher{any: true}, nil
	}

	values := map[int]struct{}{}
	segments := strings.Split(field, ",")
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			return cronFieldMatcher{}, errors.New("empty segment")
		}
		if err := appendCronSegmentValues(values, segment, minValue, maxValue, normalizeSunday); err != nil {
			return cronFieldMatcher{}, err
		}
	}
	if len(values) == 0 {
		return cronFieldMatcher{}, errors.New("no values parsed")
	}
	return cronFieldMatcher{
		any:    false,
		values: values,
	}, nil
}

func appendCronSegmentValues(values map[int]struct{}, segment string, minValue, maxValue int, normalizeSunday bool) error {
	base := segment
	step := 1
	if strings.Contains(segment, "/") {
		stepParts := strings.SplitN(segment, "/", 2)
		if len(stepParts) != 2 {
			return fmt.Errorf("invalid step segment %q", segment)
		}
		base = strings.TrimSpace(stepParts[0])
		stepRaw := strings.TrimSpace(stepParts[1])
		parsedStep, err := strconv.Atoi(stepRaw)
		if err != nil || parsedStep <= 0 {
			return fmt.Errorf("invalid step value %q", stepRaw)
		}
		step = parsedStep
	}

	base = strings.TrimSpace(base)
	if base == "" {
		base = "*"
	}

	start := minValue
	end := maxValue
	switch {
	case base == "*":
		// keep full range
	case strings.Contains(base, "-"):
		rangeParts := strings.SplitN(base, "-", 2)
		if len(rangeParts) != 2 {
			return fmt.Errorf("invalid range segment %q", segment)
		}
		rangeStart, err := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
		if err != nil {
			return fmt.Errorf("invalid range start %q", rangeParts[0])
		}
		rangeEnd, err := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
		if err != nil {
			return fmt.Errorf("invalid range end %q", rangeParts[1])
		}
		start = normalizeCronValue(rangeStart, normalizeSunday)
		end = normalizeCronValue(rangeEnd, normalizeSunday)
	default:
		singleValue, err := strconv.Atoi(base)
		if err != nil {
			return fmt.Errorf("invalid value %q", base)
		}
		start = normalizeCronValue(singleValue, normalizeSunday)
		end = start
		if step > 1 {
			end = maxValue
		}
	}

	if start < minValue || start > maxValue {
		return fmt.Errorf("value %d out of range [%d,%d]", start, minValue, maxValue)
	}
	if end < minValue || end > maxValue {
		return fmt.Errorf("value %d out of range [%d,%d]", end, minValue, maxValue)
	}
	if end < start {
		return fmt.Errorf("invalid range %d-%d", start, end)
	}

	for value := start; value <= end; value += step {
		normalizedValue := normalizeCronValue(value, normalizeSunday)
		if normalizedValue < minValue || normalizedValue > maxValue {
			continue
		}
		values[normalizedValue] = struct{}{}
	}
	return nil
}

func normalizeCronValue(value int, normalizeSunday bool) int {
	if normalizeSunday && value == 7 {
		return 0
	}
	return value
}
