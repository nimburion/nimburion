package app

import (
	"fmt"
	"strings"

	"github.com/nimburion/nimburion/pkg/core/feature"
)

// DependencyError reports a problem resolving declared feature dependencies.
//
// It is a typed error (rather than a panic) so callers can distinguish wiring
// mistakes from runtime failures. Kind narrows the cause; the offending
// feature names are carried for diagnostics.
type DependencyError struct {
	Kind    DependencyErrorKind
	Feature string
	// Missing is set for ErrMissingDependency: the unknown dependency name.
	Missing string
	// Cycle is set for ErrDependencyCycle: the feature names forming the cycle.
	Cycle []string
}

// DependencyErrorKind enumerates the dependency resolution failure modes.
type DependencyErrorKind int

const (
	// ErrMissingDependency means a feature declared a dependency on a name that
	// no registered feature provides.
	ErrMissingDependency DependencyErrorKind = iota
	// ErrDependencyCycle means declared dependencies form a cycle.
	ErrDependencyCycle
)

// Error implements the error interface.
func (e *DependencyError) Error() string {
	switch e.Kind {
	case ErrMissingDependency:
		return fmt.Sprintf("feature %q depends on unknown feature %q", e.Feature, e.Missing)
	case ErrDependencyCycle:
		return fmt.Sprintf("feature dependency cycle detected: %s", strings.Join(e.Cycle, " -> "))
	default:
		return "feature dependency error"
	}
}

// orderFeatures returns features in dependency order: every feature appears
// after all features it declares via the optional feature.DependencyDeclaring
// interface. Features that do not declare dependencies retain their original
// registration order relative to one another (stable topological sort).
//
// nil features are dropped. A dependency on an unknown name or a cycle is
// reported as a *DependencyError rather than panicking.
func orderFeatures(features []feature.Feature) ([]feature.Feature, error) {
	// Filter nils while preserving order.
	filtered := make([]feature.Feature, 0, len(features))
	for _, f := range features {
		if f != nil {
			filtered = append(filtered, f)
		}
	}

	// Index features by name; later registrations win on duplicate names, which
	// matches append-style override semantics elsewhere in the framework.
	byName := make(map[string]feature.Feature, len(filtered))
	for _, f := range filtered {
		byName[f.Name()] = f
	}

	const (
		unvisited = 0
		visiting  = 1
		visited   = 2
	)
	state := make(map[string]int, len(filtered))
	ordered := make([]feature.Feature, 0, len(filtered))
	var stack []string

	var visit func(f feature.Feature) error
	visit = func(f feature.Feature) error {
		name := f.Name()
		switch state[name] {
		case visited:
			return nil
		case visiting:
			// Found a back edge: build the cycle path for diagnostics.
			cycle := append([]string(nil), stack...)
			for i, n := range cycle {
				if n == name {
					cycle = cycle[i:]
					break
				}
			}
			cycle = append(cycle, name)
			return &DependencyError{Kind: ErrDependencyCycle, Cycle: cycle}
		}

		state[name] = visiting
		stack = append(stack, name)

		if declarer, ok := f.(feature.DependencyDeclaring); ok {
			for _, dep := range declarer.DependsOn() {
				depFeature, exists := byName[dep]
				if !exists {
					return &DependencyError{Kind: ErrMissingDependency, Feature: name, Missing: dep}
				}
				if err := visit(depFeature); err != nil {
					return err
				}
			}
		}

		stack = stack[:len(stack)-1]
		state[name] = visited
		ordered = append(ordered, f)
		return nil
	}

	for _, f := range filtered {
		if err := visit(f); err != nil {
			return nil, err
		}
	}

	return ordered, nil
}
