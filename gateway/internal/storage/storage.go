// Package storage abstracts attachment blob storage behind a small interface so
// the dev local-disk backend can be swapped for an S3-compatible one in prod
// without touching the upload/download handlers.
package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Storage stores and retrieves attachment blobs by opaque key.
type Storage interface {
	Put(key string, r io.Reader) error
	Open(key string) (io.ReadCloser, error)
	Delete(key string) error
}

// LocalDisk stores blobs as files under a root directory. Keys are validated to
// stay within root (no path traversal). NOT for production scale — use an
// S3-compatible backend there.
type LocalDisk struct {
	root string
}

// NewLocalDisk creates the root directory if needed and returns the backend.
func NewLocalDisk(root string) (*LocalDisk, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create storage root: %w", err)
	}
	return &LocalDisk{root: root}, nil
}

func (l *LocalDisk) path(key string) (string, error) {
	// Reject any key that escapes root (e.g. "../"); keys are server-generated
	// uuids, this is defense in depth.
	clean := filepath.Join(l.root, filepath.Base(key))
	if filepath.Dir(clean) != filepath.Clean(l.root) {
		return "", fmt.Errorf("invalid storage key")
	}
	return clean, nil
}

func (l *LocalDisk) Put(key string, r io.Reader) error {
	p, err := l.path(key)
	if err != nil {
		return err
	}
	f, err := os.Create(p)
	if err != nil {
		return fmt.Errorf("create blob: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("write blob: %w", err)
	}
	return nil
}

func (l *LocalDisk) Open(key string) (io.ReadCloser, error) {
	p, err := l.path(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("open blob: %w", err)
	}
	return f, nil
}

func (l *LocalDisk) Delete(key string) error {
	p, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil {
		return fmt.Errorf("delete blob: %w", err)
	}
	return nil
}
