package steer

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	inboxFilename = "inbox.md"
	delimiter     = "\n---\n"
)

// InboxMessage is one coordinator signal read from the inbox.
type InboxMessage struct {
	Message   string `json:"message"`
	Timestamp string `json:"ts"`
}

// CreateInbox creates the inbox file at dispatch start.
// Uses O_CREATE|O_EXCL — returns nil if file already exists (idempotent for reruns).
func CreateInbox(artifactDir string) error {
	f, err := os.OpenFile(inboxPath(artifactDir), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return fmt.Errorf("create inbox: %w", err)
	}
	return f.Close()
}

// WriteInbox appends one NDJSON-encoded message under an exclusive flock.
func WriteInbox(artifactDir, message string) error {
	f, err := os.OpenFile(inboxPath(artifactDir), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("open inbox for append: %w", err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock inbox: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	payload, err := json.Marshal(InboxMessage{
		Message:   message,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return fmt.Errorf("marshal inbox message: %w", err)
	}
	payload = append(payload, '\n')
	if _, err := f.Write(payload); err != nil {
		return fmt.Errorf("append inbox message: %w", err)
	}
	return nil
}

// ReadInbox reads all messages from the inbox and clears it atomically.
// Uses flock(LOCK_EX) to ensure exclusivity with concurrent writers.
// Returns slice of messages, or nil if inbox is empty.
func ReadInbox(artifactDir string) ([]InboxMessage, error) {
	f, err := os.OpenFile(inboxPath(artifactDir), os.O_RDWR, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open inbox for read: %w", err)
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return nil, fmt.Errorf("lock inbox: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("read inbox: %w", err)
	}
	if err := f.Truncate(0); err != nil {
		return nil, fmt.Errorf("truncate inbox: %w", err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("rewind inbox: %w", err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	messages := parseInboxData(data)
	if len(messages) == 0 {
		return nil, nil
	}
	return messages, nil
}

// HasMessages checks if the inbox has any content without locking.
// Fast path: just check file size > 0.
func HasMessages(artifactDir string) bool {
	info, err := os.Stat(inboxPath(artifactDir))
	if err != nil {
		return false
	}
	return info.Size() > 0
}

func inboxPath(artifactDir string) string {
	return filepath.Join(artifactDir, inboxFilename)
}

func parseInboxData(data []byte) []InboxMessage {
	if messages, ok := parseNDJSONLines(string(data)); ok {
		return messages
	}
	return parseLegacyBlocks(string(data))
}

func parseNDJSONLines(data string) ([]InboxMessage, bool) {
	lines := strings.Split(data, "\n")
	messages := make([]InboxMessage, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		var message InboxMessage
		if err := json.Unmarshal([]byte(trimmed), &message); err != nil {
			return nil, false
		}
		messages = append(messages, message)
	}
	return messages, true
}

func parseLegacyBlocks(data string) []InboxMessage {
	parts := strings.Split(data, delimiter)
	messages := make([]InboxMessage, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		if ndjsonMessages, ok := parseNDJSONLines(part); ok && len(ndjsonMessages) > 0 {
			messages = append(messages, ndjsonMessages...)
			continue
		}
		messages = append(messages, InboxMessage{Message: part})
	}
	return messages
}
