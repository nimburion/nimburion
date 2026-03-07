package saga

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type Status string

const (
	StatusPending      Status = "pending"
	StatusRunning      Status = "running"
	StatusCompleted    Status = "completed"
	StatusCompensating Status = "compensating"
	StatusCompensated  Status = "compensated"
	StatusFailed       Status = "failed"
)

type Step struct {
	Name       string
	Execute    func(context.Context) error
	Compensate func(context.Context) error
}

func (s *Step) Validate() error {
	if s == nil {
		return errors.New("saga step is nil")
	}
	if s.Name == "" {
		return errors.New("saga step name is required")
	}
	if s.Execute == nil {
		return errors.New("saga step execute is required")
	}
	return nil
}

type Definition struct {
	Name  string
	Steps []Step
}

func (d *Definition) Validate() error {
	if d == nil {
		return errors.New("saga definition is nil")
	}
	if d.Name == "" {
		return errors.New("saga name is required")
	}
	if len(d.Steps) == 0 {
		return errors.New("saga must have at least one step")
	}
	seen := map[string]struct{}{}
	for idx := range d.Steps {
		if err := d.Steps[idx].Validate(); err != nil {
			return fmt.Errorf("saga step %d: %w", idx, err)
		}
		if _, ok := seen[d.Steps[idx].Name]; ok {
			return fmt.Errorf("duplicate saga step %q", d.Steps[idx].Name)
		}
		seen[d.Steps[idx].Name] = struct{}{}
	}
	return nil
}

type Event struct {
	SagaName   string
	StepName   string
	Status     Status
	OccurredAt time.Time
	Error      string
}

type Observer interface {
	OnEvent(context.Context, Event) error
}

type Result struct {
	Status         Status
	CompletedSteps []string
	Compensated    []string
}

type Runner struct {
	definition Definition
	observer   Observer
}

func NewRunner(definition Definition, observer Observer) (*Runner, error) {
	if err := definition.Validate(); err != nil {
		return nil, err
	}
	return &Runner{definition: definition, observer: observer}, nil
}

func (r *Runner) Run(ctx context.Context) (Result, error) {
	if ctx == nil {
		return Result{}, errors.New("context is required")
	}

	result := Result{Status: StatusRunning}
	completed := make([]Step, 0, len(r.definition.Steps))
	_ = r.emit(ctx, "", StatusRunning, nil)

	for _, step := range r.definition.Steps {
		if err := step.Execute(ctx); err != nil {
			_ = r.emit(ctx, step.Name, StatusFailed, err)
			result.Status = StatusCompensating
			_ = r.emit(ctx, step.Name, StatusCompensating, err)

			for idx := len(completed) - 1; idx >= 0; idx-- {
				done := completed[idx]
				if done.Compensate == nil {
					continue
				}
				if compErr := done.Compensate(ctx); compErr != nil {
					_ = r.emit(ctx, done.Name, StatusFailed, compErr)
					return result, errors.Join(err, compErr)
				}
				result.Compensated = append(result.Compensated, done.Name)
			}
			result.Status = StatusCompensated
			_ = r.emit(ctx, step.Name, StatusCompensated, err)
			return result, err
		}

		completed = append(completed, step)
		result.CompletedSteps = append(result.CompletedSteps, step.Name)
		_ = r.emit(ctx, step.Name, StatusRunning, nil)
	}

	result.Status = StatusCompleted
	_ = r.emit(ctx, "", StatusCompleted, nil)
	return result, nil
}

func (r *Runner) emit(ctx context.Context, stepName string, status Status, err error) error {
	if r.observer == nil {
		return nil
	}
	event := Event{
		SagaName:   r.definition.Name,
		StepName:   stepName,
		Status:     status,
		OccurredAt: time.Now().UTC(),
	}
	if err != nil {
		event.Error = err.Error()
	}
	return r.observer.OnEvent(ctx, event)
}
