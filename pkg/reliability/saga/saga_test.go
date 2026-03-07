package saga

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type recordingObserver struct {
	events []Event
}

func (o *recordingObserver) OnEvent(_ context.Context, event Event) error {
	o.events = append(o.events, event)
	return nil
}

func TestRunner_CompletesSaga(t *testing.T) {
	observer := &recordingObserver{}
	runner, err := NewRunner(Definition{
		Name: "checkout",
		Steps: []Step{
			{Name: "reserve_inventory", Execute: func(context.Context) error { return nil }},
			{Name: "charge_payment", Execute: func(context.Context) error { return nil }},
		},
	}, observer)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	result, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("run saga: %v", err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", result.Status)
	}
	if !reflect.DeepEqual(result.CompletedSteps, []string{"reserve_inventory", "charge_payment"}) {
		t.Fatalf("unexpected completed steps: %+v", result.CompletedSteps)
	}
}

func TestRunner_CompensatesCompletedStepsOnFailure(t *testing.T) {
	var compensated []string
	runner, err := NewRunner(Definition{
		Name: "checkout",
		Steps: []Step{
			{
				Name:       "reserve_inventory",
				Execute:    func(context.Context) error { return nil },
				Compensate: func(context.Context) error { compensated = append(compensated, "reserve_inventory"); return nil },
			},
			{
				Name:    "charge_payment",
				Execute: func(context.Context) error { return errors.New("payment failed") },
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	result, err := runner.Run(context.Background())
	if err == nil {
		t.Fatal("expected saga failure")
	}
	if result.Status != StatusCompensated {
		t.Fatalf("expected compensated, got %s", result.Status)
	}
	if !reflect.DeepEqual(compensated, []string{"reserve_inventory"}) {
		t.Fatalf("unexpected compensation order: %+v", compensated)
	}
}
