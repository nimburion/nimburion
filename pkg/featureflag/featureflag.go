// Package featureflag provides shared runtime feature-gating and rollout-safety contracts.
package featureflag

import (
	"context"
	"fmt"
	"math"
	"sync"
)

// Kind identifies the typed surface of a flag definition.
type Kind string

const (
	// KindBool represents a boolean feature flag.
	KindBool Kind = "bool"
	// KindString represents a string feature flag.
	KindString Kind = "string"
	// KindInt represents an integer feature flag.
	KindInt Kind = "int"
)

// Source identifies where one evaluation value came from.
type Source string

const (
	// SourceDefault indicates the definition default was used.
	SourceDefault Source = "default"
	// SourceProvider indicates the value came from the configured provider.
	SourceProvider Source = "provider"
	// SourceSafeDefault indicates the provider failed and the safe default was used.
	SourceSafeDefault Source = "safe_default"
)

// EventType identifies one emitted feature-flag runtime event.
type EventType string

const (
	// Feature-flag runtime event types emitted by the registry.
	EventTypeProviderChanged EventType = "feature_flag.provider_changed"
	EventTypeDefinitionAdded EventType = "feature_flag.definition_added"
	EventTypeEvaluated       EventType = "feature_flag.evaluated"
)

// Event describes one runtime event emitted by the registry.
type Event struct {
	Type    EventType
	Key     string
	Kind    Kind
	Source  Source
	Outcome string
	Error   string
	Target  TargetContext
}

// Observer receives emitted events.
type Observer interface {
	OnFeatureFlagEvent(ctx context.Context, event Event)
}

// ObserverFunc adapts a function to the Observer interface.
type ObserverFunc func(context.Context, Event)

// OnFeatureFlagEvent forwards one event.
func (f ObserverFunc) OnFeatureFlagEvent(ctx context.Context, event Event) {
	f(ctx, event)
}

// TargetContext carries runtime attributes used by providers for targeting.
type TargetContext struct {
	AppName     string
	Environment string
	UserID      string
	TenantID    string
	Attributes  map[string]string
}

// Provider resolves feature flag values for one target context.
type Provider interface {
	Lookup(ctx context.Context, key string, target TargetContext) (any, bool, error)
}

// ProviderFunc adapts a function to the Provider interface.
type ProviderFunc func(context.Context, string, TargetContext) (any, bool, error)

// Lookup resolves one feature flag value.
func (f ProviderFunc) Lookup(ctx context.Context, key string, target TargetContext) (any, bool, error) {
	return f(ctx, key, target)
}

// BoolDefinition describes one boolean feature flag.
type BoolDefinition struct {
	Key         string
	Default     bool
	Description string
}

// StringDefinition describes one string feature flag.
type StringDefinition struct {
	Key         string
	Default     string
	Description string
}

// IntDefinition describes one integer feature flag.
type IntDefinition struct {
	Key         string
	Default     int
	Description string
}

// BoolEvaluation contains one typed boolean evaluation result.
type BoolEvaluation struct {
	Key         string        `json:"key"`
	Kind        Kind          `json:"kind"`
	Value       bool          `json:"value"`
	Default     bool          `json:"default"`
	Source      Source        `json:"source"`
	Description string        `json:"description,omitempty"`
	Target      TargetContext `json:"target"`
	Error       string        `json:"error,omitempty"`
}

// StringEvaluation contains one typed string evaluation result.
type StringEvaluation struct {
	Key         string        `json:"key"`
	Kind        Kind          `json:"kind"`
	Value       string        `json:"value"`
	Default     string        `json:"default"`
	Source      Source        `json:"source"`
	Description string        `json:"description,omitempty"`
	Target      TargetContext `json:"target"`
	Error       string        `json:"error,omitempty"`
}

// IntEvaluation contains one typed integer evaluation result.
type IntEvaluation struct {
	Key         string        `json:"key"`
	Kind        Kind          `json:"kind"`
	Value       int           `json:"value"`
	Default     int           `json:"default"`
	Source      Source        `json:"source"`
	Description string        `json:"description,omitempty"`
	Target      TargetContext `json:"target"`
	Error       string        `json:"error,omitempty"`
}

// Registry stores typed flag definitions and resolves them through one provider.
type Registry struct {
	mu                sync.RWMutex
	provider          Provider
	boolDefinitions   map[string]BoolDefinition
	stringDefinitions map[string]StringDefinition
	intDefinitions    map[string]IntDefinition
	observers         []Observer
}

// NewRegistry creates an empty feature flag registry.
func NewRegistry() *Registry {
	return &Registry{
		boolDefinitions:   make(map[string]BoolDefinition),
		stringDefinitions: make(map[string]StringDefinition),
		intDefinitions:    make(map[string]IntDefinition),
	}
}

// AddObserver registers one observer for runtime events.
func (r *Registry) AddObserver(observer Observer) {
	if observer == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.observers = append(r.observers, observer)
}

// SetProvider replaces the registry provider used for evaluations.
func (r *Registry) SetProvider(provider Provider) {
	r.mu.Lock()
	r.provider = provider
	r.mu.Unlock()

	r.emit(context.Background(), Event{Type: EventTypeProviderChanged, Outcome: "provider_set"})
}

// RegisterBool registers one boolean flag definition.
func (r *Registry) RegisterBool(def BoolDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.validateRegistration(def.Key, KindBool); err != nil {
		return err
	}
	r.boolDefinitions[def.Key] = def
	r.emitLocked(context.Background(), Event{Type: EventTypeDefinitionAdded, Key: def.Key, Kind: KindBool, Outcome: "registered"})
	return nil
}

// RegisterString registers one string flag definition.
func (r *Registry) RegisterString(def StringDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.validateRegistration(def.Key, KindString); err != nil {
		return err
	}
	r.stringDefinitions[def.Key] = def
	r.emitLocked(context.Background(), Event{Type: EventTypeDefinitionAdded, Key: def.Key, Kind: KindString, Outcome: "registered"})
	return nil
}

// RegisterInt registers one integer flag definition.
func (r *Registry) RegisterInt(def IntDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.validateRegistration(def.Key, KindInt); err != nil {
		return err
	}
	r.intDefinitions[def.Key] = def
	r.emitLocked(context.Background(), Event{Type: EventTypeDefinitionAdded, Key: def.Key, Kind: KindInt, Outcome: "registered"})
	return nil
}

func (r *Registry) validateRegistration(key string, kind Kind) error {
	if key == "" {
		return fmt.Errorf("feature flag key is required")
	}
	if kind != KindBool {
		if _, ok := r.boolDefinitions[key]; ok {
			return fmt.Errorf("feature flag %q already registered as %s", key, KindBool)
		}
	}
	if kind != KindString {
		if _, ok := r.stringDefinitions[key]; ok {
			return fmt.Errorf("feature flag %q already registered as %s", key, KindString)
		}
	}
	if kind != KindInt {
		if _, ok := r.intDefinitions[key]; ok {
			return fmt.Errorf("feature flag %q already registered as %s", key, KindInt)
		}
	}
	return nil
}

// EvalBool evaluates one boolean flag for the target context with safe-default semantics.
func (r *Registry) EvalBool(ctx context.Context, key string, target TargetContext) BoolEvaluation {
	r.mu.RLock()
	def, ok := r.boolDefinitions[key]
	provider := r.provider
	r.mu.RUnlock()

	result := BoolEvaluation{
		Key:         key,
		Kind:        KindBool,
		Default:     def.Default,
		Value:       def.Default,
		Source:      SourceDefault,
		Description: def.Description,
		Target:      target,
	}
	if !ok {
		result.Error = fmt.Sprintf("feature flag %q is not registered", key)
		r.emit(ctx, Event{Type: EventTypeEvaluated, Key: key, Kind: KindBool, Source: SourceSafeDefault, Outcome: "unregistered", Error: result.Error, Target: target})
		return result
	}
	if provider == nil {
		r.emit(ctx, Event{Type: EventTypeEvaluated, Key: key, Kind: KindBool, Source: result.Source, Outcome: "default_used", Target: target})
		return result
	}

	raw, found, err := provider.Lookup(ctx, key, target)
	if err != nil {
		result.Source = SourceSafeDefault
		result.Error = err.Error()
		r.emit(ctx, Event{Type: EventTypeEvaluated, Key: key, Kind: KindBool, Source: result.Source, Outcome: "provider_error", Error: result.Error, Target: target})
		return result
	}
	if !found {
		r.emit(ctx, Event{Type: EventTypeEvaluated, Key: key, Kind: KindBool, Source: result.Source, Outcome: "not_found", Target: target})
		return result
	}

	value, castOK := raw.(bool)
	if !castOK {
		result.Source = SourceSafeDefault
		result.Error = fmt.Sprintf("feature flag %q expected bool value, got %T", key, raw)
		r.emit(ctx, Event{Type: EventTypeEvaluated, Key: key, Kind: KindBool, Source: result.Source, Outcome: "type_mismatch", Error: result.Error, Target: target})
		return result
	}
	result.Value = value
	result.Source = SourceProvider
	r.emit(ctx, Event{Type: EventTypeEvaluated, Key: key, Kind: KindBool, Source: result.Source, Outcome: "provider_value", Target: target})
	return result
}

// EvalString evaluates one string flag for the target context with safe-default semantics.
func (r *Registry) EvalString(ctx context.Context, key string, target TargetContext) StringEvaluation {
	r.mu.RLock()
	def, ok := r.stringDefinitions[key]
	provider := r.provider
	r.mu.RUnlock()

	result := StringEvaluation{
		Key:         key,
		Kind:        KindString,
		Default:     def.Default,
		Value:       def.Default,
		Source:      SourceDefault,
		Description: def.Description,
		Target:      target,
	}
	if !ok {
		result.Error = fmt.Sprintf("feature flag %q is not registered", key)
		r.emit(ctx, Event{Type: EventTypeEvaluated, Key: key, Kind: KindString, Source: SourceSafeDefault, Outcome: "unregistered", Error: result.Error, Target: target})
		return result
	}
	if provider == nil {
		r.emit(ctx, Event{Type: EventTypeEvaluated, Key: key, Kind: KindString, Source: result.Source, Outcome: "default_used", Target: target})
		return result
	}

	raw, found, err := provider.Lookup(ctx, key, target)
	if err != nil {
		result.Source = SourceSafeDefault
		result.Error = err.Error()
		r.emit(ctx, Event{Type: EventTypeEvaluated, Key: key, Kind: KindString, Source: result.Source, Outcome: "provider_error", Error: result.Error, Target: target})
		return result
	}
	if !found {
		r.emit(ctx, Event{Type: EventTypeEvaluated, Key: key, Kind: KindString, Source: result.Source, Outcome: "not_found", Target: target})
		return result
	}

	value, castOK := raw.(string)
	if !castOK {
		result.Source = SourceSafeDefault
		result.Error = fmt.Sprintf("feature flag %q expected string value, got %T", key, raw)
		r.emit(ctx, Event{Type: EventTypeEvaluated, Key: key, Kind: KindString, Source: result.Source, Outcome: "type_mismatch", Error: result.Error, Target: target})
		return result
	}
	result.Value = value
	result.Source = SourceProvider
	r.emit(ctx, Event{Type: EventTypeEvaluated, Key: key, Kind: KindString, Source: result.Source, Outcome: "provider_value", Target: target})
	return result
}

// EvalInt evaluates one integer flag for the target context with safe-default semantics.
func (r *Registry) EvalInt(ctx context.Context, key string, target TargetContext) IntEvaluation {
	r.mu.RLock()
	def, ok := r.intDefinitions[key]
	provider := r.provider
	r.mu.RUnlock()

	result := IntEvaluation{
		Key:         key,
		Kind:        KindInt,
		Default:     def.Default,
		Value:       def.Default,
		Source:      SourceDefault,
		Description: def.Description,
		Target:      target,
	}
	if !ok {
		result.Error = fmt.Sprintf("feature flag %q is not registered", key)
		r.emit(ctx, Event{Type: EventTypeEvaluated, Key: key, Kind: KindInt, Source: SourceSafeDefault, Outcome: "unregistered", Error: result.Error, Target: target})
		return result
	}
	if provider == nil {
		r.emit(ctx, Event{Type: EventTypeEvaluated, Key: key, Kind: KindInt, Source: result.Source, Outcome: "default_used", Target: target})
		return result
	}

	raw, found, err := provider.Lookup(ctx, key, target)
	if err != nil {
		result.Source = SourceSafeDefault
		result.Error = err.Error()
		r.emit(ctx, Event{Type: EventTypeEvaluated, Key: key, Kind: KindInt, Source: result.Source, Outcome: "provider_error", Error: result.Error, Target: target})
		return result
	}
	if !found {
		r.emit(ctx, Event{Type: EventTypeEvaluated, Key: key, Kind: KindInt, Source: result.Source, Outcome: "not_found", Target: target})
		return result
	}

	value, castErr := castInt(raw)
	if castErr != nil {
		result.Source = SourceSafeDefault
		result.Error = fmt.Sprintf("feature flag %q %s", key, castErr.Error())
		r.emit(ctx, Event{Type: EventTypeEvaluated, Key: key, Kind: KindInt, Source: result.Source, Outcome: "type_mismatch", Error: result.Error, Target: target})
		return result
	}
	result.Value = value
	result.Source = SourceProvider
	r.emit(ctx, Event{Type: EventTypeEvaluated, Key: key, Kind: KindInt, Source: result.Source, Outcome: "provider_value", Target: target})
	return result
}

func castInt(raw any) (int, error) {
	switch value := raw.(type) {
	case int:
		return value, nil
	case int32:
		return int(value), nil
	case int64:
		if int64(int(value)) != value {
			return 0, fmt.Errorf("expected int value, got out-of-range int64")
		}
		return int(value), nil
	case float64:
		if math.Trunc(value) != value {
			return 0, fmt.Errorf("expected int value, got non-integer float64")
		}
		if value > float64(math.MaxInt) || value < float64(math.MinInt) {
			return 0, fmt.Errorf("expected int value, got out-of-range float64")
		}
		return int(value), nil
	default:
		return 0, fmt.Errorf("expected int value, got %T", raw)
	}
}

func (r *Registry) emit(ctx context.Context, event Event) {
	r.mu.RLock()
	observers := append([]Observer(nil), r.observers...)
	r.mu.RUnlock()
	for _, observer := range observers {
		observer.OnFeatureFlagEvent(ctx, event)
	}
}

func (r *Registry) emitLocked(ctx context.Context, event Event) {
	for _, observer := range r.observers {
		observer.OnFeatureFlagEvent(ctx, event)
	}
}

// Snapshot returns the typed flag definitions currently registered.
func (r *Registry) Snapshot() map[string]map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snapshot := make(map[string]map[string]any, len(r.boolDefinitions)+len(r.stringDefinitions)+len(r.intDefinitions))
	for key, def := range r.boolDefinitions {
		snapshot[key] = map[string]any{
			"kind":        KindBool,
			"default":     def.Default,
			"description": def.Description,
		}
	}
	for key, def := range r.stringDefinitions {
		snapshot[key] = map[string]any{
			"kind":        KindString,
			"default":     def.Default,
			"description": def.Description,
		}
	}
	for key, def := range r.intDefinitions {
		snapshot[key] = map[string]any{
			"kind":        KindInt,
			"default":     def.Default,
			"description": def.Description,
		}
	}
	return snapshot
}
