//go:build windows

package steer

import (
	"errors"
	"os"
	"path/filepath"
)

var ErrUnsupported = errors.New("fifo unsupported on this platform")

func Path(artifactDir string) string {
	return filepath.Join(artifactDir, "stdin.pipe")
}

func Create(path string) error {
	return ErrUnsupported
}

func OpenReadNonblock(path string) (*os.File, error) {
	return nil, ErrUnsupported
}

func OpenWriteNonblock(path string) (*os.File, error) {
	return nil, ErrUnsupported
}

func Remove(path string) error {
	return ErrUnsupported
}
