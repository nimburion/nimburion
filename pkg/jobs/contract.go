package jobs

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nimburion/nimburion/pkg/eventbus"
)

// Job content type constants
const (
	// DefaultContentType is the default content type for job payloads
	DefaultContentType = "application/json"
)

// Job header constants
const (
	// HeaderJobID is the unique job identifier
	HeaderJobID = "job_id"
	// HeaderJobName is the job handler name
	HeaderJobName = "job_name"
	// HeaderJobQueue is the queue name
	HeaderJobQueue = "job_queue"
	// HeaderJobTenantID is the tenant identifier
	HeaderJobTenantID = "job_tenant_id"
	// HeaderJobCorrelationID is the correlation identifier for tracing
	HeaderJobCorrelationID = "job_correlation_id"
	// HeaderJobIdempotencyKey ensures job is processed only once
	HeaderJobIdempotencyKey = "job_idempotency_key"
	// HeaderJobRunAt is the scheduled execution time
	HeaderJobRunAt = "job_run_at"
	// HeaderJobAttempt is the current attempt number
	HeaderJobAttempt = "job_attempt"
	// HeaderJobMaxAttempts is the maximum retry attempts
	HeaderJobMaxAttempts = "job_max_attempts"
)

// Job describes one logical application workload unit.
type Job struct {
	ID             string
	Name           string
	Queue          string
	Payload        []byte
	Headers        map[string]string
	ContentType    string
	TenantID       string
	CorrelationID  string
	IdempotencyKey string
	PartitionKey   string
	RunAt          time.Time
	Attempt        int
	MaxAttempts    int
	CreatedAt      time.Time
}

// Validate checks the required fields used by runtime behavior.
func (j *Job) Validate() error {
	if j == nil {
		return jobsError(ErrValidation, "job is nil")
	}
	if strings.TrimSpace(j.ID) == "" {
		return jobsError(ErrValidation, "job id is required")
	}
	if strings.TrimSpace(j.Name) == "" {
		return jobsError(ErrValidation, "job name is required")
	}
	if strings.TrimSpace(j.Queue) == "" {
		return jobsError(ErrValidation, "job queue is required")
	}
	if len(j.Payload) == 0 {
		return jobsError(ErrValidation, "job payload is required")
	}
	if j.Attempt < 0 {
		return jobsError(ErrValidation, "job attempt must be >= 0")
	}
	if j.MaxAttempts < 0 {
		return jobsError(ErrValidation, "job max attempts must be >= 0")
	}
	if j.MaxAttempts > 0 && j.Attempt > j.MaxAttempts {
		return jobsError(ErrValidation, "job attempt cannot exceed max attempts")
	}
	return nil
}

// ToEventBusMessage converts a job into an eventbus message while keeping transport-specific
// concerns inside eventbus primitives.
func (j *Job) ToEventBusMessage() (*eventbus.Message, error) {
	if err := j.Validate(); err != nil {
		return nil, err
	}

	headers := cloneHeaders(j.Headers)
	createdAt := j.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	runAt := j.RunAt.UTC()
	if runAt.IsZero() {
		runAt = createdAt
	}

	headers[HeaderJobID] = strings.TrimSpace(j.ID)
	headers[HeaderJobName] = strings.TrimSpace(j.Name)
	headers[HeaderJobQueue] = strings.TrimSpace(j.Queue)
	headers[HeaderJobRunAt] = runAt.Format(time.RFC3339Nano)
	headers[HeaderJobAttempt] = strconv.Itoa(j.Attempt)
	headers[HeaderJobMaxAttempts] = strconv.Itoa(j.MaxAttempts)

	if trimmed := strings.TrimSpace(j.TenantID); trimmed != "" {
		headers[HeaderJobTenantID] = trimmed
		// Keep compatibility with eventbus envelope conventions.
		if headers["tenant_id"] == "" {
			headers["tenant_id"] = trimmed
		}
	}
	if trimmed := strings.TrimSpace(j.CorrelationID); trimmed != "" {
		headers[HeaderJobCorrelationID] = trimmed
		if headers["correlation_id"] == "" {
			headers["correlation_id"] = trimmed
		}
	}
	if trimmed := strings.TrimSpace(j.IdempotencyKey); trimmed != "" {
		headers[HeaderJobIdempotencyKey] = trimmed
	}

	contentType := strings.TrimSpace(j.ContentType)
	if contentType == "" {
		contentType = DefaultContentType
	}

	key := strings.TrimSpace(j.PartitionKey)
	if key == "" {
		key = JobPartitionKey(j.TenantID, j.Name, j.ID)
	}

	return &eventbus.Message{
		ID:          strings.TrimSpace(j.ID),
		Key:         key,
		Value:       cloneBytes(j.Payload),
		Headers:     headers,
		ContentType: contentType,
		Timestamp:   createdAt,
	}, nil
}

// JobFromEventBusMessage decodes a job from a consumed eventbus message.
func JobFromEventBusMessage(msg *eventbus.Message) (*Job, error) {
	return JobFromEventBusMessageWithQueue(msg, "")
}

// JobFromEventBusMessageWithQueue decodes a job and applies a fallback queue when missing.
func JobFromEventBusMessageWithQueue(msg *eventbus.Message, fallbackQueue string) (*Job, error) {
	if msg == nil {
		return nil, jobsError(ErrValidation, "message is nil")
	}
	if strings.TrimSpace(msg.ID) == "" {
		return nil, jobsError(ErrValidation, "message id is required")
	}
	if len(msg.Value) == 0 {
		return nil, jobsError(ErrValidation, "message payload is empty")
	}

	headers := cloneHeaders(msg.Headers)
	job := &Job{
		ID:             strings.TrimSpace(msg.ID),
		Name:           strings.TrimSpace(headers[HeaderJobName]),
		Queue:          strings.TrimSpace(headers[HeaderJobQueue]),
		Payload:        cloneBytes(msg.Value),
		Headers:        headers,
		ContentType:    strings.TrimSpace(msg.ContentType),
		TenantID:       strings.TrimSpace(firstNonEmpty(headers[HeaderJobTenantID], headers["tenant_id"])),
		CorrelationID:  strings.TrimSpace(firstNonEmpty(headers[HeaderJobCorrelationID], headers["correlation_id"])),
		IdempotencyKey: strings.TrimSpace(headers[HeaderJobIdempotencyKey]),
		PartitionKey:   strings.TrimSpace(msg.Key),
		CreatedAt:      msg.Timestamp.UTC(),
	}

	if job.ContentType == "" {
		job.ContentType = DefaultContentType
	}
	if job.Queue == "" {
		job.Queue = strings.TrimSpace(fallbackQueue)
	}
	if runAtHeader := strings.TrimSpace(headers[HeaderJobRunAt]); runAtHeader != "" {
		runAt, err := time.Parse(time.RFC3339Nano, runAtHeader)
		if err != nil {
			return nil, errors.Join(jobsError(ErrValidation, fmt.Sprintf("invalid %s header", HeaderJobRunAt)), err)
		}
		job.RunAt = runAt.UTC()
	}
	if job.RunAt.IsZero() {
		job.RunAt = job.CreatedAt
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now().UTC()
		if job.RunAt.IsZero() {
			job.RunAt = job.CreatedAt
		}
	}

	if attemptHeader := strings.TrimSpace(headers[HeaderJobAttempt]); attemptHeader != "" {
		attempt, err := strconv.Atoi(attemptHeader)
		if err != nil {
			return nil, errors.Join(jobsError(ErrValidation, fmt.Sprintf("invalid %s header", HeaderJobAttempt)), err)
		}
		job.Attempt = attempt
	}
	if maxAttemptsHeader := strings.TrimSpace(headers[HeaderJobMaxAttempts]); maxAttemptsHeader != "" {
		maxAttempts, err := strconv.Atoi(maxAttemptsHeader)
		if err != nil {
			return nil, errors.Join(jobsError(ErrValidation, fmt.Sprintf("invalid %s header", HeaderJobMaxAttempts)), err)
		}
		job.MaxAttempts = maxAttempts
	}

	if err := job.Validate(); err != nil {
		return nil, err
	}
	return job, nil
}

// JobPartitionKey returns a stable partition key to preserve ordering when possible.
func JobPartitionKey(tenantID, jobName, jobID string) string {
	tenantID = strings.TrimSpace(tenantID)
	jobName = strings.TrimSpace(jobName)
	jobID = strings.TrimSpace(jobID)

	switch {
	case tenantID != "" && jobName != "":
		return tenantID + ":" + jobName
	case jobName != "" && jobID != "":
		return jobName + ":" + jobID
	case jobID != "":
		return jobID
	default:
		return jobName
	}
}

// MarshalPayloadJSON marshals payload using the same conventions used by eventbus JSON payloads.
func MarshalPayloadJSON(payload any) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.Join(jobsError(ErrValidation, "marshal job payload failed"), err)
	}
	return data, nil
}

func cloneHeaders(input map[string]string) map[string]string {
	if len(input) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func cloneBytes(input []byte) []byte {
	if len(input) == 0 {
		return nil
	}
	out := make([]byte, len(input))
	copy(out, input)
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
