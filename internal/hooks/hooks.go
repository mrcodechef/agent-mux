// Package hooks provides pre-dispatch validation and event-level pattern detection.
// Phase 2 implementation. This package defines the interface only.
package hooks

// Validator checks dispatch specs and events against pattern lists.
type Validator interface {
	// ValidatePreDispatch checks prompt text against deny/warn lists.
	// Returns (denied bool, warnings []string).
	ValidatePreDispatch(prompt string) (bool, []string)

	// ValidateEvent checks a harness event against deny/warn lists.
	// Returns (denied bool, warnings []string).
	ValidateEvent(eventType, content string) (bool, []string)
}

// NoopValidator is a no-op implementation for Phase 1.
type NoopValidator struct{}

func NewNoopValidator() *NoopValidator { return &NoopValidator{} }
func (v *NoopValidator) ValidatePreDispatch(prompt string) (bool, []string) { return false, nil }
func (v *NoopValidator) ValidateEvent(eventType, content string) (bool, []string) { return false, nil }
