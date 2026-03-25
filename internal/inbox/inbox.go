// Package inbox provides the coordinator mailbox interface.
// Phase 2 implementation. This package defines the interface only.
package inbox

// Mailbox is the interface for coordinator message delivery.
// Phase 2 will implement file-based inbox with POSIX O_APPEND writes.
type Mailbox interface {
	// Check reads and clears pending messages. Returns empty string if none.
	Check() (string, error)

	// Send appends a message to the inbox.
	Send(message string) error

	// Close cleans up resources.
	Close() error
}

// NoopMailbox is a no-op implementation for Phase 1.
type NoopMailbox struct{}

func NewNoopMailbox() *NoopMailbox { return &NoopMailbox{} }
func (m *NoopMailbox) Check() (string, error) { return "", nil }
func (m *NoopMailbox) Send(message string) error { return nil }
func (m *NoopMailbox) Close() error { return nil }
