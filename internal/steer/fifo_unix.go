//go:build !windows

package steer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

var ErrUnsupported = errors.New("fifo unsupported on this platform")

func Path(artifactDir string) string {
	return filepath.Join(artifactDir, "stdin.pipe")
}

func Create(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale fifo: %w", err)
	}
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		return fmt.Errorf("mkfifo %s: %w", path, err)
	}
	return nil
}

func OpenReadNonblock(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

func OpenWriteNonblock(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

func Remove(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
