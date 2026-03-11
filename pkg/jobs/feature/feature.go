// Package jobsfeature wires CLI commands for the jobs subsystem.
package jobsfeature

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/nimburion/nimburion/pkg/config"
	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	eventbusconfig "github.com/nimburion/nimburion/pkg/eventbus/config"
	schemavalidationconfig "github.com/nimburion/nimburion/pkg/eventbus/schema/config"
	"github.com/nimburion/nimburion/pkg/jobs"
	jobsfamilyconfig "github.com/nimburion/nimburion/pkg/jobs/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

const (
	commandPolicyAnnotationPrefix = "policies."
	defaultCommandPolicyContext   = "run"
	policyManual                  = "manual"
	policyRun                     = "run"
	policyScheduled               = "scheduled"
)

// ConfigLoader loads config and logger for contributed jobs commands.
type ConfigLoader func(cmd *cobra.Command) (*config.Config, logger.Logger, error)

// RuntimeFactory builds a jobs runtime from family-owned config.
type RuntimeFactory func(
	cfg jobsfamilyconfig.Config,
	eventBusCfg eventbusconfig.Config,
	validationCfg schemavalidationconfig.KafkaValidationConfig,
	log logger.Logger,
) (jobs.Runtime, error)

// BackendFactory builds a jobs backend from family-owned config.
type BackendFactory func(
	cfg jobsfamilyconfig.Config,
	eventBusCfg eventbusconfig.Config,
	validationCfg schemavalidationconfig.KafkaValidationConfig,
	log logger.Logger,
) (jobs.Backend, error)

// WorkerConfigurer applies application-specific worker registrations and hooks.
type WorkerConfigurer func(cfg *config.Config, log logger.Logger, worker jobs.Worker) error

// CommandFeatureOptions configures the jobs CLI feature wiring.
type CommandFeatureOptions struct {
	LoadConfig      ConfigLoader
	RuntimeFactory  RuntimeFactory
	BackendFactory  BackendFactory
	ConfigureWorker WorkerConfigurer
}

type commandFeature struct {
	command *cobra.Command
}

func (f commandFeature) Name() string { return "jobs" }

func (f commandFeature) Contributions() corefeature.Contributions {
	if f.command == nil {
		return corefeature.Contributions{}
	}
	return corefeature.Contributions{
		ConfigExtensions: []corefeature.ConfigExtension{
			{Name: "jobs", Extension: &jobsfamilyconfig.Extension{}},
		},
		CommandRegistrations: []corefeature.CommandContribution{
			{Name: f.command.Name(), Command: f.command},
		},
	}
}

// NewCommandFeature returns the feature that wires jobs CLI commands and config.
func NewCommandFeature(opts CommandFeatureOptions) corefeature.Feature {
	if opts.LoadConfig == nil {
		return nil
	}

	buildRuntime := func(cfg *config.Config, log logger.Logger) (jobs.Runtime, error) {
		runtimeFactory := opts.RuntimeFactory
		if runtimeFactory == nil {
			runtimeFactory = jobs.NewRuntimeFromConfigWithValidation
		}
		return runtimeFactory(cfg.Jobs, cfg.EventBus, cfg.Validation.Kafka, log)
	}

	buildBackend := func(cfg *config.Config, log logger.Logger) (jobs.Backend, error) {
		backendFactory := opts.BackendFactory
		if backendFactory == nil {
			backendFactory = jobs.NewBackendFromConfigWithValidation
		}
		return backendFactory(cfg.Jobs, cfg.EventBus, cfg.Validation.Kafka, log)
	}

	jobsCmd := &cobra.Command{Use: "jobs", Short: "Jobs runtime commands"}
	setCommandPolicy(jobsCmd, policyRun)

	var (
		queues         []string
		concurrency    int
		leaseTTL       time.Duration
		reserveTO      time.Duration
		stopTO         time.Duration
		maxAttempts    int
		initialBackoff time.Duration
		maxBackoff     time.Duration
		attemptTO      time.Duration
		dlqEnabled     bool
		dlqSuffix      string
	)
	workerCmd := &cobra.Command{
		Use:   "worker",
		Short: "Run jobs worker",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.ConfigureWorker == nil {
				return errors.New("configure worker callback is required for jobs worker command")
			}
			cfg, log, err := opts.LoadConfig(cmd)
			if err != nil {
				return err
			}
			backend, err := buildBackend(cfg, log)
			if err != nil {
				return wrapCommandError("create jobs backend", err)
			}
			defer closeWithLog(log, "jobs backend", backend.Close)

			resolvedConcurrency := cfg.Jobs.Worker.Concurrency
			if cmd.Flags().Changed("concurrency") {
				resolvedConcurrency = concurrency
			}
			resolvedLeaseTTL := cfg.Jobs.Worker.LeaseTTL
			if cmd.Flags().Changed("lease-ttl") {
				resolvedLeaseTTL = leaseTTL
			}
			resolvedReserveTO := cfg.Jobs.Worker.ReserveTimeout
			if cmd.Flags().Changed("reserve-timeout") {
				resolvedReserveTO = reserveTO
			}
			resolvedStopTO := cfg.Jobs.Worker.StopTimeout
			if cmd.Flags().Changed("stop-timeout") {
				resolvedStopTO = stopTO
			}
			resolvedMaxAttempts := cfg.Jobs.Retry.MaxAttempts
			if cmd.Flags().Changed("max-attempts") {
				resolvedMaxAttempts = maxAttempts
			}
			resolvedInitialBackoff := cfg.Jobs.Retry.InitialBackoff
			if cmd.Flags().Changed("initial-backoff") {
				resolvedInitialBackoff = initialBackoff
			}
			resolvedMaxBackoff := cfg.Jobs.Retry.MaxBackoff
			if cmd.Flags().Changed("max-backoff") {
				resolvedMaxBackoff = maxBackoff
			}
			resolvedAttemptTO := cfg.Jobs.Retry.AttemptTimeout
			if cmd.Flags().Changed("attempt-timeout") {
				resolvedAttemptTO = attemptTO
			}
			resolvedDLQEnabled := cfg.Jobs.DLQ.Enabled
			if cmd.Flags().Changed("dlq-enabled") {
				resolvedDLQEnabled = dlqEnabled
			}
			resolvedDLQSuffix := cfg.Jobs.DLQ.QueueSuffix
			if cmd.Flags().Changed("dlq-suffix") {
				resolvedDLQSuffix = dlqSuffix
			}
			workerQueues := resolveWorkerQueues(queues, cfg.Jobs.DefaultQueue)
			worker, err := jobs.NewWorker(backend, log, jobs.WorkerConfig{
				Queues:         workerQueues,
				Concurrency:    resolvedConcurrency,
				LeaseTTL:       resolvedLeaseTTL,
				ReserveTimeout: resolvedReserveTO,
				StopTimeout:    resolvedStopTO,
				Retry: jobs.RetryPolicy{
					MaxAttempts:    resolvedMaxAttempts,
					InitialBackoff: resolvedInitialBackoff,
					MaxBackoff:     resolvedMaxBackoff,
					AttemptTimeout: resolvedAttemptTO,
				},
				DLQ: jobs.DLQPolicy{Enabled: resolvedDLQEnabled, QueueSuffix: resolvedDLQSuffix},
			})
			if err != nil {
				return wrapCommandError("create worker", err)
			}
			if err := opts.ConfigureWorker(cfg, log, worker); err != nil {
				return wrapCommandError("configure jobs worker", err)
			}
			runCtx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return worker.Start(runCtx)
		},
	}
	setCommandPolicy(workerCmd, policyScheduled)
	workerCmd.Flags().StringSliceVar(&queues, "queue", []string{}, "queue names to consume (repeatable)")
	workerCmd.Flags().IntVar(&concurrency, "concurrency", 1, "number of workers per queue")
	workerCmd.Flags().DurationVar(&leaseTTL, "lease-ttl", jobs.DefaultLeaseTTL, "lease duration per reserved job")
	workerCmd.Flags().DurationVar(&reserveTO, "reserve-timeout", jobs.DefaultWorkerReserveTimeout, "reserve call timeout")
	workerCmd.Flags().DurationVar(&stopTO, "stop-timeout", jobs.DefaultWorkerStopTimeout, "graceful stop timeout")
	workerCmd.Flags().IntVar(&maxAttempts, "max-attempts", jobs.DefaultWorkerMaxAttempts, "maximum processing attempts per job")
	workerCmd.Flags().DurationVar(&initialBackoff, "initial-backoff", jobs.DefaultWorkerInitialBackoff, "retry initial backoff")
	workerCmd.Flags().DurationVar(&maxBackoff, "max-backoff", jobs.DefaultWorkerMaxBackoff, "retry max backoff")
	workerCmd.Flags().DurationVar(&attemptTO, "attempt-timeout", jobs.DefaultWorkerAttemptTimeout, "timeout for a single job execution")
	workerCmd.Flags().BoolVar(&dlqEnabled, "dlq-enabled", true, "enable dead-letter queue routing")
	workerCmd.Flags().StringVar(&dlqSuffix, "dlq-suffix", jobs.DefaultDLQSuffix, "suffix for dead-letter queues")
	jobsCmd.AddCommand(workerCmd)

	var (
		enqueueName           string
		enqueueQueue          string
		enqueuePayload        string
		enqueueTenantID       string
		enqueueCorrelationID  string
		enqueueIdempotencyKey string
		enqueueRunAt          string
		enqueueMaxAttempts    int
		enqueueHeaders        []string
	)
	enqueueCmd := &cobra.Command{
		Use:   "enqueue",
		Short: "Enqueue a job manually",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, log, err := opts.LoadConfig(cmd)
			if err != nil {
				return err
			}
			runtime, err := buildRuntime(cfg, log)
			if err != nil {
				return wrapCommandError("create jobs runtime", err)
			}
			defer closeWithLog(log, "jobs runtime", runtime.Close)
			jobName := strings.TrimSpace(enqueueName)
			if jobName == "" {
				return errors.New("job name is required")
			}
			queue := strings.TrimSpace(enqueueQueue)
			if queue == "" {
				queue = strings.TrimSpace(cfg.Jobs.DefaultQueue)
			}
			if queue == "" {
				queue = "default"
			}
			payload := strings.TrimSpace(enqueuePayload)
			if payload == "" {
				payload = "{}"
			}
			headers, err := parseStringMapFlag(enqueueHeaders)
			if err != nil {
				return err
			}
			runAt := time.Time{}
			if raw := strings.TrimSpace(enqueueRunAt); raw != "" {
				parsedRunAt, parseErr := time.Parse(time.RFC3339, raw)
				if parseErr != nil {
					return fmt.Errorf("invalid --run-at value %q: %w", raw, parseErr)
				}
				runAt = parsedRunAt.UTC()
			}
			job := &jobs.Job{
				ID:             uuid.NewString(),
				Name:           jobName,
				Queue:          queue,
				Payload:        []byte(payload),
				Headers:        headers,
				ContentType:    jobs.DefaultContentType,
				TenantID:       strings.TrimSpace(enqueueTenantID),
				CorrelationID:  strings.TrimSpace(enqueueCorrelationID),
				IdempotencyKey: strings.TrimSpace(enqueueIdempotencyKey),
				RunAt:          runAt,
				MaxAttempts:    enqueueMaxAttempts,
				CreatedAt:      time.Now().UTC(),
			}
			if err := runtime.Enqueue(cmd.Context(), job); err != nil {
				return wrapCommandError("enqueue job", err)
			}
			return writeCommandf(cmd, "enqueued job %s on queue %s\n", job.ID, job.Queue)
		},
	}
	setCommandPolicy(enqueueCmd, policyManual)
	enqueueCmd.Flags().StringVar(&enqueueName, "name", "", "job name")
	enqueueCmd.Flags().StringVar(&enqueueQueue, "queue", "", "target queue (default: jobs.default_queue)")
	enqueueCmd.Flags().StringVar(&enqueuePayload, "payload", "{}", "job payload as JSON string")
	enqueueCmd.Flags().StringVar(&enqueueTenantID, "tenant-id", "", "tenant id")
	enqueueCmd.Flags().StringVar(&enqueueCorrelationID, "correlation-id", "", "correlation id")
	enqueueCmd.Flags().StringVar(&enqueueIdempotencyKey, "idempotency-key", "", "idempotency key")
	enqueueCmd.Flags().StringVar(&enqueueRunAt, "run-at", "", "schedule run time in RFC3339 format")
	enqueueCmd.Flags().IntVar(&enqueueMaxAttempts, "max-attempts", 0, "max attempts override for this job")
	enqueueCmd.Flags().StringSliceVar(&enqueueHeaders, "header", []string{}, "job header in key=value format (repeatable)")
	mustMarkFlagRequired(enqueueCmd, "name")
	jobsCmd.AddCommand(enqueueCmd)

	dlqCmd := &cobra.Command{Use: "dlq", Short: "Dead-letter queue operations"}
	setCommandPolicy(dlqCmd, policyManual)
	var (
		dlqListQueue string
		dlqListLimit int
	)
	dlqListCmd := &cobra.Command{
		Use:   "list",
		Short: "List dead-letter queue entries",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, log, err := opts.LoadConfig(cmd)
			if err != nil {
				return err
			}
			backend, err := buildBackend(cfg, log)
			if err != nil {
				return wrapCommandError("create jobs backend", err)
			}
			defer closeWithLog(log, "jobs backend", backend.Close)
			dlqStore, ok := backend.(jobs.DLQStore)
			if !ok {
				return errors.New("dlq operations are not supported by current jobs backend")
			}
			entries, err := dlqStore.ListDLQ(cmd.Context(), dlqListQueue, dlqListLimit)
			if err != nil {
				return wrapCommandError("list dlq entries", err)
			}
			data, err := json.MarshalIndent(entries, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal dlq entries: %w", err)
			}
			return writeCommandln(cmd, string(data))
		},
	}
	setCommandPolicy(dlqListCmd, policyManual)
	dlqListCmd.Flags().StringVar(&dlqListQueue, "queue", "", "original queue name")
	dlqListCmd.Flags().IntVar(&dlqListLimit, "limit", 50, "max number of entries to list")
	mustMarkFlagRequired(dlqListCmd, "queue")
	dlqCmd.AddCommand(dlqListCmd)

	var (
		dlqReplayQueue string
		dlqReplayIDs   []string
	)
	dlqReplayCmd := &cobra.Command{
		Use:   "replay",
		Short: "Replay dead-letter queue entries",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, log, err := opts.LoadConfig(cmd)
			if err != nil {
				return err
			}
			backend, err := buildBackend(cfg, log)
			if err != nil {
				return wrapCommandError("create jobs backend", err)
			}
			defer closeWithLog(log, "jobs backend", backend.Close)
			dlqStore, ok := backend.(jobs.DLQStore)
			if !ok {
				return errors.New("dlq operations are not supported by current jobs backend")
			}
			if len(dlqReplayIDs) == 0 {
				return errors.New("at least one --id value is required")
			}
			replayed, err := dlqStore.ReplayDLQ(cmd.Context(), dlqReplayQueue, dlqReplayIDs)
			if err != nil {
				return wrapCommandError("replay dlq entries", err)
			}
			return writeCommandf(cmd, "replayed %d entries from queue %s\n", replayed, dlqReplayQueue)
		},
	}
	setCommandPolicy(dlqReplayCmd, policyManual)
	dlqReplayCmd.Flags().StringVar(&dlqReplayQueue, "queue", "", "original queue name")
	dlqReplayCmd.Flags().StringSliceVar(&dlqReplayIDs, "id", []string{}, "DLQ entry id to replay (repeatable)")
	mustMarkFlagRequired(dlqReplayCmd, "queue")
	dlqCmd.AddCommand(dlqReplayCmd)

	jobsCmd.AddCommand(dlqCmd)
	return commandFeature{command: jobsCmd}
}

func resolveWorkerQueues(flagQueues []string, defaultQueue string) []string {
	queues := make([]string, 0, len(flagQueues))
	for _, queue := range flagQueues {
		trimmed := strings.TrimSpace(queue)
		if trimmed != "" {
			queues = append(queues, trimmed)
		}
	}
	if len(queues) > 0 {
		return queues
	}
	if trimmed := strings.TrimSpace(defaultQueue); trimmed != "" {
		return []string{trimmed}
	}
	return []string{"default"}
}

func wrapCommandError(message string, err error) error {
	return fmt.Errorf("%s: %w", message, canonicalizeJobsCommandError(err))
}

func canonicalizeJobsCommandError(err error) error {
	switch {
	case errors.Is(err, jobs.ErrValidation):
		return coreerrors.NewValidationWithCode("validation.jobs", err.Error(), nil, nil)
	case errors.Is(err, jobs.ErrConflict):
		return coreerrors.New("jobs.conflict", nil, err).WithMessage(err.Error()).WithHTTPStatus(409)
	case errors.Is(err, jobs.ErrNotFound):
		return coreerrors.New("jobs.not_found", nil, err).WithMessage(err.Error()).WithHTTPStatus(404)
	case errors.Is(err, jobs.ErrRetryable):
		return coreerrors.NewRetryable(err.Error(), err).WithDetails(map[string]interface{}{"family": "jobs"})
	case errors.Is(err, jobs.ErrInvalidArgument):
		return coreerrors.New("argument.jobs.invalid", nil, err).WithMessage(err.Error()).WithHTTPStatus(400)
	case errors.Is(err, jobs.ErrNotInitialized):
		return coreerrors.NewNotInitialized(err.Error(), err).WithDetails(map[string]interface{}{"family": "jobs"})
	case errors.Is(err, jobs.ErrClosed):
		return coreerrors.NewClosed(err.Error(), err).WithDetails(map[string]interface{}{"family": "jobs"})
	default:
		return err
	}
}

func parseStringMapFlag(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	result := make(map[string]string, len(values))
	for _, item := range values {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			return nil, fmt.Errorf("invalid key=value pair %q", item)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("invalid key=value pair %q", item)
		}
		result[key] = strings.TrimSpace(value)
	}
	return result, nil
}

func writeCommandf(cmd *cobra.Command, format string, args ...any) error {
	_, err := fmt.Fprintf(cmd.OutOrStdout(), format, args...)
	return err
}

func writeCommandln(cmd *cobra.Command, value string) error {
	_, err := fmt.Fprintln(cmd.OutOrStdout(), value)
	return err
}

func closeWithLog(log logger.Logger, name string, closeFn func() error) {
	if closeErr := closeFn(); closeErr != nil && log != nil {
		log.Error("failed to close "+name, "error", closeErr)
	}
}

func mustMarkFlagRequired(cmd *cobra.Command, name string) {
	if err := cmd.MarkFlagRequired(name); err != nil {
		panic(fmt.Sprintf("mark %s as required: %v", name, err))
	}
}

func setCommandPolicy(cmd *cobra.Command, policy string) {
	if cmd == nil {
		return
	}
	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}
	cmd.Annotations[commandPolicyAnnotationPrefix+defaultCommandPolicyContext] = policy
}
