// Package recovery provides artifact scanning and dispatch continuation.
// Phase 2 implementation. This package defines the interface only.
package recovery

// Manager handles --recover flows: scanning artifact dirs and building
// continuation prompts.
type Manager interface {
	// ScanArtifacts reads the artifact directory for a previous dispatch.
	ScanArtifacts(dispatchID string) ([]string, error)

	// BuildContinuationPrompt constructs a prompt that includes prior work context.
	BuildContinuationPrompt(dispatchID, userPrompt string) (string, error)
}

// NoopManager is a no-op implementation for Phase 1.
type NoopManager struct{}

func NewNoopManager() *NoopManager { return &NoopManager{} }
func (m *NoopManager) ScanArtifacts(dispatchID string) ([]string, error) { return nil, nil }
func (m *NoopManager) BuildContinuationPrompt(dispatchID, userPrompt string) (string, error) {
	return userPrompt, nil
}
