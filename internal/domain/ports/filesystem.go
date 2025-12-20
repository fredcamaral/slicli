package ports

import (
	"io"
	"os"
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
