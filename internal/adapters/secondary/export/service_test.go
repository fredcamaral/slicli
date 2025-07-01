package export

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/test/builders"
)

// MockRenderer implements the Renderer interface for testing
type MockRenderer struct {
	mock.Mock
}

func (m *MockRenderer) Render(ctx context.Context, presentation *entities.Presentation, options *ExportOptions) (*ExportResult, error) {
	args := m.Called(ctx, presentation, options)
	if result := args.Get(0); result != nil {
		return result.(*ExportResult), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRenderer) Supports(format ExportFormat) bool {
	args := m.Called(format)
	return args.Bool(0)
}

func (m *MockRenderer) GetMimeType() string {
	args := m.Called()
	return args.String(0)
}

func TestNewService(t *testing.T) {
	t.Run("creates service with default temp directory", func(t *testing.T) {
		service, err := NewService("")
		require.NoError(t, err)
		assert.NotNil(t, service)
		assert.Equal(t, os.TempDir(), service.tmpDir)
		assert.Len(t, service.renderers, 4) // HTML, PDF, Images, Markdown
	})

	t.Run("creates service with custom temp directory", func(t *testing.T) {
		customDir := "/tmp/slicli-test"
		service, err := NewService(customDir)
		require.NoError(t, err)
		assert.NotNil(t, service)
		assert.Equal(t, customDir, service.tmpDir)
	})

	t.Run("creates temp directory if it doesn't exist", func(t *testing.T) {
		// Use a unique directory name to avoid conflicts
		customDir := filepath.Join(os.TempDir(), "slicli-test-"+time.Now().Format("20060102150405"))
		defer func() { _ = os.RemoveAll(customDir) }()

		service, err := NewService(customDir)
		require.NoError(t, err)
		assert.NotNil(t, service)

		// Verify directory was created
		info, err := os.Stat(customDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})
}

func TestService_RegisterRenderer(t *testing.T) {
	service, err := NewService("")
	require.NoError(t, err)

	mockRenderer := new(MockRenderer)
	service.RegisterRenderer(FormatPDF, mockRenderer)

	assert.Equal(t, mockRenderer, service.renderers[FormatPDF])
}

func TestService_Export(t *testing.T) {
	presentation := builders.NewPresentationBuilder().
		WithTitle("Test Export").
		WithSlideCount(3).
		Build()

	t.Run("successful export", func(t *testing.T) {
		testService, err := NewService("")
		require.NoError(t, err)

		mockRenderer := new(MockRenderer)
		expectedResult := &ExportResult{
			Success:    true,
			Format:     string(FormatHTML),
			OutputPath: "/tmp/test.html",
			FileSize:   1024,
			PageCount:  3,
		}

		mockRenderer.On("Render", mock.Anything, presentation, mock.Anything).Return(expectedResult, nil)
		testService.RegisterRenderer(FormatHTML, mockRenderer)

		options := &ExportOptions{
			Format:     FormatHTML,
			OutputPath: "/tmp/test.html",
		}

		result, err := testService.Export(context.Background(), presentation, options)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.Equal(t, string(FormatHTML), result.Format)
		assert.NotEmpty(t, result.Duration)
		assert.NotZero(t, result.GeneratedAt)

		mockRenderer.AssertExpectations(t)
	})

	t.Run("unsupported format", func(t *testing.T) {
		testService, err := NewService("")
		require.NoError(t, err)

		options := &ExportOptions{
			Format:     ExportFormat("unsupported"),
			OutputPath: "/tmp/test.unsupported",
		}

		result, err := testService.Export(context.Background(), presentation, options)
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "unsupported export format")
	})

	t.Run("renderer error", func(t *testing.T) {
		testService, err := NewService("")
		require.NoError(t, err)

		mockRenderer := new(MockRenderer)
		renderError := errors.New("rendering failed")

		mockRenderer.On("Render", mock.Anything, presentation, mock.Anything).Return(nil, renderError)
		testService.RegisterRenderer(FormatHTML, mockRenderer)

		options := &ExportOptions{
			Format:     FormatHTML,
			OutputPath: "/tmp/test.html",
		}

		result, err := testService.Export(context.Background(), presentation, options)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rendering failed")
		assert.NotNil(t, result)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "rendering failed")

		mockRenderer.AssertExpectations(t)
	})

	t.Run("invalid output directory", func(t *testing.T) {
		testService, err := NewService("")
		require.NoError(t, err)

		mockRenderer := new(MockRenderer)
		testService.RegisterRenderer(FormatHTML, mockRenderer)

		options := &ExportOptions{
			Format:     FormatHTML,
			OutputPath: "/invalid/nonexistent/deeply/nested/path/test.html",
		}

		result, err := testService.Export(context.Background(), presentation, options)
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "failed to create output directory")
	})
}

func TestService_validateOptions(t *testing.T) {
	service, err := NewService("")
	require.NoError(t, err)

	tests := []struct {
		name        string
		options     *ExportOptions
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil options",
			options:     nil,
			expectError: true,
			errorMsg:    "export options cannot be nil",
		},
		{
			name: "empty format",
			options: &ExportOptions{
				OutputPath: "/tmp/test.html",
			},
			expectError: true,
			errorMsg:    "export format is required",
		},
		{
			name: "empty output path",
			options: &ExportOptions{
				Format: FormatHTML,
			},
			expectError: true,
			errorMsg:    "output path is required",
		},
		{
			name: "invalid quality",
			options: &ExportOptions{
				Format:     FormatHTML,
				OutputPath: "/tmp/test.html",
				Quality:    "ultra",
			},
			expectError: true,
			errorMsg:    "invalid quality setting",
		},
		{
			name: "invalid page size",
			options: &ExportOptions{
				Format:     FormatPDF,
				OutputPath: "/tmp/test.pdf",
				PageSize:   "Tabloid",
			},
			expectError: true,
			errorMsg:    "invalid page size",
		},
		{
			name: "invalid orientation",
			options: &ExportOptions{
				Format:      FormatPDF,
				OutputPath:  "/tmp/test.pdf",
				Orientation: "diagonal",
			},
			expectError: true,
			errorMsg:    "invalid orientation",
		},
		{
			name: "valid options",
			options: &ExportOptions{
				Format:      FormatHTML,
				OutputPath:  "/tmp/test.html",
				Quality:     "high",
				PageSize:    "A4",
				Orientation: "landscape",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateOptions(tt.options)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestService_GetSupportedFormats(t *testing.T) {
	service, err := NewService("")
	require.NoError(t, err)

	formats := service.GetSupportedFormats()
	assert.Len(t, formats, 4)
	assert.Contains(t, formats, FormatHTML)
	assert.Contains(t, formats, FormatPDF)
	assert.Contains(t, formats, FormatImages)
	assert.Contains(t, formats, FormatMarkdown)
}

func TestService_GetTempDir(t *testing.T) {
	customDir := filepath.Join(os.TempDir(), "custom-temp")
	defer func() { _ = os.RemoveAll(customDir) }()

	service, err := NewService(customDir)
	require.NoError(t, err)

	assert.Equal(t, customDir, service.GetTempDir())
}

func TestService_CreateTempFile(t *testing.T) {
	service, err := NewService("")
	require.NoError(t, err)

	file, err := service.CreateTempFile("test", ".html")
	require.NoError(t, err)
	defer func() { _ = os.Remove(file.Name()) }()
	defer func() { _ = file.Close() }()

	assert.Contains(t, file.Name(), "test")
	assert.Contains(t, file.Name(), ".html")
}

func TestService_CleanupTempFiles(t *testing.T) {
	// Create a unique temp directory for this test to avoid interference
	uniqueDir := filepath.Join(os.TempDir(), "slicli-export-test-"+time.Now().Format("20060102150405"))
	defer func() { _ = os.RemoveAll(uniqueDir) }()

	service, err := NewService(uniqueDir)
	require.NoError(t, err)

	// Create a test file in the temp directory with proper prefix
	testFile := filepath.Join(service.tmpDir, "slicli-export-test.html")
	err = os.WriteFile(testFile, []byte("test"), 0600)
	require.NoError(t, err)

	// Cleanup should not remove recent files
	err = service.CleanupTempFiles(time.Hour)
	require.NoError(t, err)

	// File should still exist
	_, err = os.Stat(testFile)
	assert.NoError(t, err)

	// Cleanup with zero duration should remove the file
	err = service.CleanupTempFiles(0)
	require.NoError(t, err)

	// File should be removed
	_, err = os.Stat(testFile)
	assert.True(t, os.IsNotExist(err), "File should have been removed by cleanup")
}

func TestGetFileSize(t *testing.T) {
	// Create a temporary file
	content := []byte("test content for size calculation")
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.Write(content)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Test getting file size
	size, err := GetFileSize(tmpFile.Name())
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), size)

	// Test with non-existent file
	_, err = GetFileSize("/nonexistent/file.txt")
	assert.Error(t, err)
}

func TestCopyFile(t *testing.T) {
	// Create source file
	srcContent := []byte("test content for copying")
	srcFile, err := os.CreateTemp("", "src-*.txt")
	require.NoError(t, err)
	defer func() { _ = os.Remove(srcFile.Name()) }()

	_, err = srcFile.Write(srcContent)
	require.NoError(t, err)
	_ = srcFile.Close()

	// Test successful copy
	dstFile := filepath.Join(os.TempDir(), "dst-test.txt")
	defer func() { _ = os.Remove(dstFile) }()

	err = CopyFile(srcFile.Name(), dstFile)
	require.NoError(t, err)

	// Verify copy
	dstContent, err := os.ReadFile(dstFile)
	require.NoError(t, err)
	assert.Equal(t, srcContent, dstContent)

	// Test with invalid source
	err = CopyFile("/nonexistent/file.txt", dstFile)
	assert.Error(t, err)

	// Test with directory traversal in source
	err = CopyFile("../../../etc/passwd", dstFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid source path")

	// Test with directory traversal in destination
	err = CopyFile(srcFile.Name(), "../../../tmp/malicious.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid destination path")
}

func TestValidateFilePath(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty path",
			path:        "",
			expectError: true,
			errorMsg:    "empty path",
		},
		{
			name:        "directory traversal with ..",
			path:        "../../../etc/passwd",
			expectError: true,
			errorMsg:    "path contains directory traversal",
		},
		{
			name:        "directory traversal in relative path",
			path:        "temp/../../../etc/passwd",
			expectError: true,
			errorMsg:    "path contains directory traversal",
		},
		{
			name:        "absolute path",
			path:        "/tmp/valid/file.txt",
			expectError: false,
		},
		{
			name:        "relative path",
			path:        "valid/file.txt",
			expectError: false,
		},
		{
			name:        "current directory",
			path:        "./file.txt",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFilePath(tt.path)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
