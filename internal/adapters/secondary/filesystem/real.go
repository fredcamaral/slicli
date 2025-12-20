package filesystem

import (
	"os"
	"path/filepath"

	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// RealFileSystem implements FileSystem using actual OS operations.
type RealFileSystem struct{}

// NewRealFileSystem creates a new real file system implementation
func NewRealFileSystem() ports.FileSystem {
	return &RealFileSystem{}
}

// Open opens a file for reading
func (fs *RealFileSystem) Open(name string) (ports.File, error) {
	// #nosec G304 - File paths are controlled by the application for reading presentation files
	// This filesystem interface is used for legitimate file operations in a CLI tool
	return os.Open(name)
}

// Create creates a file for writing
func (fs *RealFileSystem) Create(name string) (ports.File, error) {
	// #nosec G304 - File paths are controlled by the application for creating output files
	// This filesystem interface is used for legitimate file operations in a CLI tool
	return os.Create(name)
}

// Remove removes a file
func (fs *RealFileSystem) Remove(name string) error {
	return os.Remove(name)
}

// RemoveAll removes a directory and all its contents
func (fs *RealFileSystem) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// Mkdir creates a directory
func (fs *RealFileSystem) Mkdir(name string, perm os.FileMode) error {
	return os.Mkdir(name, perm)
}

// MkdirAll creates a directory and all parent directories
func (fs *RealFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Stat returns file information
func (fs *RealFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

// Exists checks if a file or directory exists
func (fs *RealFileSystem) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ReadFile reads the entire file content
func (fs *RealFileSystem) ReadFile(filename string) ([]byte, error) {
	// #nosec G304 - File paths are controlled by the application for reading configuration and content files
	// This filesystem interface is used for legitimate file operations in a CLI tool
	return os.ReadFile(filename)
}

// WriteFile writes data to a file
func (fs *RealFileSystem) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return os.WriteFile(filename, data, perm)
}

// TempDir returns the system temporary directory
func (fs *RealFileSystem) TempDir() string {
	return os.TempDir()
}

// CreateTemp creates a temporary file
func (fs *RealFileSystem) CreateTemp(dir, pattern string) (ports.File, error) {
	return os.CreateTemp(dir, pattern)
}

// Abs returns the absolute path for the given path.
func (fs *RealFileSystem) Abs(path string) (string, error) {
	return filepath.Abs(path)
}

// Walk walks the file tree
func (fs *RealFileSystem) Walk(root string, walkFn func(path string, info os.FileInfo, err error) error) error {
	return filepath.Walk(root, walkFn)
}

// Compile-time interface verification
var _ ports.FileSystem = (*RealFileSystem)(nil)
