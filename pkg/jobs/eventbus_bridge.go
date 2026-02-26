package jobs

import (
	"context"
	"errors"
	"strings"

	"github.com/nimburion/nimburion/pkg/eventbus"
)

// Handler processes consumed jobs.
type Handler func(ctx context.Context, job *Job) error

// Runtime defines a backend-agnostic jobs runtime contract.
type Runtime interface {
	Enqueue(ctx context.Context, job *Job) error
	Subscribe(ctx context.Context, queue string, handler Handler) error
	Unsubscribe(queue string) error
	HealthCheck(ctx context.Context) error
	Close() error
}

// EventBusBridge adapts an eventbus adapter to the jobs runtime contract.
// This keeps jobs and transport concerns separated while reusing existing eventbus adapters.
type EventBusBridge struct {
	bus eventbus.EventBus
}

// NewEventBusBridge creates a jobs runtime backed by eventbus.
func NewEventBusBridge(bus eventbus.EventBus) (*EventBusBridge, error) {
	if bus == nil {
		return nil, errors.New("eventbus adapter is required")
	}
	return &EventBusBridge{bus: bus}, nil
}

// Enqueue publishes a job to its queue through eventbus.
func (b *EventBusBridge) Enqueue(ctx context.Context, job *Job) error {
	if b == nil || b.bus == nil {
		return errors.New("eventbus bridge is not initialized")
	}
	if job == nil {
		return errors.New("job is required")
	}

	msg, err := job.ToEventBusMessage()
	if err != nil {
		return err
	}

	return b.bus.Publish(ctx, strings.TrimSpace(job.Queue), msg)
}

// Subscribe consumes jobs from the given queue and maps transport message to job contract.
func (b *EventBusBridge) Subscribe(ctx context.Context, queue string, handler Handler) error {
	if b == nil || b.bus == nil {
		return errors.New("eventbus bridge is not initialized")
	}
	queue = strings.TrimSpace(queue)
	if queue == "" {
		return errors.New("queue is required")
	}
	if handler == nil {
		return errors.New("handler is required")
	}

	return b.bus.Subscribe(ctx, queue, func(handlerCtx context.Context, msg *eventbus.Message) error {
		job, err := JobFromEventBusMessageWithQueue(msg, queue)
		if err != nil {
			return err
		}
		return handler(handlerCtx, job)
	})
}

// Unsubscribe removes a queue subscription.
func (b *EventBusBridge) Unsubscribe(queue string) error {
	if b == nil || b.bus == nil {
		return errors.New("eventbus bridge is not initialized")
	}
	queue = strings.TrimSpace(queue)
	if queue == "" {
		return errors.New("queue is required")
	}
	return b.bus.Unsubscribe(queue)
}

// HealthCheck checks connectivity to the underlying event bus.
func (b *EventBusBridge) HealthCheck(ctx context.Context) error {
	if b == nil || b.bus == nil {
		return errors.New("eventbus bridge is not initialized")
	}
	return b.bus.HealthCheck(ctx)
}

// Close closes the underlying event bus.
func (b *EventBusBridge) Close() error {
	if b == nil || b.bus == nil {
		return errors.New("eventbus bridge is not initialized")
	}
	return b.bus.Close()
}
