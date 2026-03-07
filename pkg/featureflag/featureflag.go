// Package featureflag provides shared runtime feature-gating and rollout-safety contracts.
package featureflag

import (
	"context"
	"fmt"
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
}

// NewRegistry creates an empty feature flag registry.
func NewRegistry() *Registry {
	return &Registry{
		boolDefinitions:   make(map[string]BoolDefinition),
		stringDefinitions: make(map[string]StringDefinition),
		intDefinitions:    make(map[string]IntDefinition),
	}
}

// SetProvider replaces the registry provider used for evaluations.
func (r *Registry) SetProvider(provider Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.provider = provider
}

// RegisterBool registers one boolean flag definition.
func (r *Registry) RegisterBool(def BoolDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.boolDefinitions[def.Key] = def
}

// RegisterString registers one string flag definition.
func (r *Registry) RegisterString(def StringDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.stringDefinitions[def.Key] = def
}

// RegisterInt registers one integer flag definition.
func (r *Registry) RegisterInt(def IntDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.intDefinitions[def.Key] = def
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
		return result
	}
	if provider == nil {
		return result
	}

	raw, found, err := provider.Lookup(ctx, key, target)
	if err != nil {
		result.Source = SourceSafeDefault
		result.Error = err.Error()
		return result
	}
	if !found {
		return result
	}

	value, castOK := raw.(bool)
	if !castOK {
		result.Source = SourceSafeDefault
		result.Error = fmt.Sprintf("feature flag %q expected bool value, got %T", key, raw)
		return result
	}
	result.Value = value
	result.Source = SourceProvider
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
		return result
	}
	if provider == nil {
		return result
	}

	raw, found, err := provider.Lookup(ctx, key, target)
	if err != nil {
		result.Source = SourceSafeDefault
		result.Error = err.Error()
		return result
	}
	if !found {
		return result
	}

	value, castOK := raw.(string)
	if !castOK {
		result.Source = SourceSafeDefault
		result.Error = fmt.Sprintf("feature flag %q expected string value, got %T", key, raw)
		return result
	}
	result.Value = value
	result.Source = SourceProvider
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
		return result
	}
	if provider == nil {
		return result
	}

	raw, found, err := provider.Lookup(ctx, key, target)
	if err != nil {
		result.Source = SourceSafeDefault
		result.Error = err.Error()
		return result
	}
	if !found {
		return result
	}

	value, castOK := raw.(int)
	if !castOK {
		result.Source = SourceSafeDefault
		result.Error = fmt.Sprintf("feature flag %q expected int value, got %T", key, raw)
		return result
	}
	result.Value = value
	result.Source = SourceProvider
	return result
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
