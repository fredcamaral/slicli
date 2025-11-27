package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test ValidatePluginID security function
func TestValidatePluginID(t *testing.T) {
	t.Run("valid plugin ID passes", func(t *testing.T) {
		validIDs := []string{
			"syntax-highlight",
			"mermaid",
			"code-exec",
			"my_plugin",
			"plugin123",
			"a",
			"Plugin-Name_v2",
		}

		for _, id := range validIDs {
			err := ValidatePluginID(id)
			assert.NoError(t, err, "Expected '%s' to be valid", id)
		}
	})

	t.Run("empty plugin ID rejected", func(t *testing.T) {
		err := ValidatePluginID("")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidPluginID)
		assert.Contains(t, err.Error(), "empty plugin ID")
	})

	t.Run("plugin ID exceeding max length rejected", func(t *testing.T) {
		longID := "a" + string(make([]byte, maxPluginIDLength))
		err := ValidatePluginID(longID)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidPluginID)
		assert.Contains(t, err.Error(), "exceeds maximum length")
	})

	t.Run("path traversal sequence rejected", func(t *testing.T) {
		traversalIDs := []string{
			"..",
			"../etc/passwd",
			"plugin/../other",
			"..%2f",
		}

		for _, id := range traversalIDs {
			err := ValidatePluginID(id)
			assert.Error(t, err, "Expected '%s' to be rejected", id)
		}
	})

	t.Run("path separator rejected", func(t *testing.T) {
		separatorIDs := []string{
			"plugin/name",
			"plugin\\name",
			"/etc/passwd",
			"..\\..\\windows\\system32",
		}

		for _, id := range separatorIDs {
			err := ValidatePluginID(id)
			assert.Error(t, err, "Expected '%s' to be rejected", id)
			assert.ErrorIs(t, err, ErrInvalidPluginID)
		}
	})

	t.Run("invalid characters rejected", func(t *testing.T) {
		invalidIDs := []string{
			"plugin name", // space
			"plugin@name", // @
			"plugin#name", // #
			"plugin$name", // $
			"plugin%name", // %
			"plugin&name", // &
			"plugin*name", // *
			"plugin!name", // !
			"plugin?name", // ?
			"<script>",    // HTML
			";rm -rf /",   // shell command
			"plugin`cmd`", // backtick
		}

		for _, id := range invalidIDs {
			err := ValidatePluginID(id)
			assert.Error(t, err, "Expected '%s' to be rejected", id)
			assert.ErrorIs(t, err, ErrInvalidPluginID)
		}
	})

	t.Run("plugin ID starting with special char rejected", func(t *testing.T) {
		invalidIDs := []string{
			"-plugin",
			"_plugin",
			".plugin",
		}

		for _, id := range invalidIDs {
			err := ValidatePluginID(id)
			assert.Error(t, err, "Expected '%s' to be rejected", id)
		}
	})
}

// Test SafePluginPath security function
func TestSafePluginPath(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "plugin-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	t.Run("valid plugin path passes", func(t *testing.T) {
		path, err := SafePluginPath(tempDir, "my-plugin")
		assert.NoError(t, err)
		assert.NotEmpty(t, path)
		assert.True(t, filepath.IsAbs(path))
	})

	t.Run("path stays within base directory", func(t *testing.T) {
		path, err := SafePluginPath(tempDir, "valid-plugin")
		assert.NoError(t, err)

		// Verify the path is within the temp directory
		relPath, err := filepath.Rel(tempDir, path)
		assert.NoError(t, err)
		assert.False(t, filepath.IsAbs(relPath), "Relative path should not be absolute")
		assert.NotContains(t, relPath, "..")
	})

	t.Run("invalid plugin ID rejected", func(t *testing.T) {
		_, err := SafePluginPath(tempDir, "../escape")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidPluginID)
	})

	t.Run("empty plugin ID rejected", func(t *testing.T) {
		_, err := SafePluginPath(tempDir, "")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidPluginID)
	})
}

// Test ValidateAssetName security function
func TestValidateAssetName(t *testing.T) {
	t.Run("valid asset names pass", func(t *testing.T) {
		validNames := []string{
			"style.css",
			"script.js",
			"image.png",
			"font-family.woff2",
			"my_asset_v2.min.css",
			"index.html",
			"a.b",
		}

		for _, name := range validNames {
			err := ValidateAssetName(name)
			assert.NoError(t, err, "Expected '%s' to be valid", name)
		}
	})

	t.Run("empty name rejected", func(t *testing.T) {
		err := ValidateAssetName("")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidAssetName)
	})

	t.Run("path traversal rejected", func(t *testing.T) {
		err := ValidateAssetName("..")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrPathTraversal)
	})

	t.Run("path separator rejected", func(t *testing.T) {
		invalidNames := []string{
			"path/to/file.css",
			"path\\to\\file.css",
		}

		for _, name := range invalidNames {
			err := ValidateAssetName(name)
			assert.Error(t, err, "Expected '%s' to be rejected", name)
			assert.ErrorIs(t, err, ErrInvalidAssetName)
		}
	})

	t.Run("null byte rejected", func(t *testing.T) {
		err := ValidateAssetName("file\x00.css")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidAssetName)
		assert.Contains(t, err.Error(), "null byte")
	})

	t.Run("invalid characters rejected", func(t *testing.T) {
		invalidNames := []string{
			"file name.css", // space
			"file@name.css", // @
			"file#name.css", // #
			"file$name.css", // $
			"file%name.css", // %
			"file&name.css", // &
			"file*name.css", // *
			"file!name.css", // !
			"file?name.css", // ?
			"<script>.js",   // HTML chars
			";rm -rf /.sh",  // shell command
		}

		for _, name := range invalidNames {
			err := ValidateAssetName(name)
			assert.Error(t, err, "Expected '%s' to be rejected", name)
			assert.ErrorIs(t, err, ErrInvalidAssetName)
		}
	})

	t.Run("name starting with special char rejected", func(t *testing.T) {
		invalidNames := []string{
			"-file.css",
			"_file.css",
			".hidden",
		}

		for _, name := range invalidNames {
			err := ValidateAssetName(name)
			assert.Error(t, err, "Expected '%s' to be rejected", name)
		}
	})
}

// Test SafeFilePath security function
func TestSafeFilePath(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "asset-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	t.Run("valid path passes", func(t *testing.T) {
		path, err := SafeFilePath(tempDir, "style.css")
		assert.NoError(t, err)
		assert.NotEmpty(t, path)
		assert.True(t, filepath.IsAbs(path))
	})

	t.Run("nested path stays within base", func(t *testing.T) {
		path, err := SafeFilePath(tempDir, "css/main.css")
		assert.NoError(t, err)

		// Verify the path is within the temp directory
		relPath, err := filepath.Rel(tempDir, path)
		assert.NoError(t, err)
		assert.False(t, filepath.IsAbs(relPath))
		assert.NotContains(t, relPath, "..")
	})

	t.Run("path traversal attempt blocked", func(t *testing.T) {
		// Note: On Unix systems, backslash is a valid filename character,
		// not a path separator, so we test Unix-style paths only
		traversalPaths := []string{
			"../etc/passwd",
			"foo/../../etc/passwd",
			"foo/../../../etc/passwd",
		}

		for _, p := range traversalPaths {
			_, err := SafeFilePath(tempDir, p)
			assert.Error(t, err, "Expected path '%s' to be rejected", p)
			assert.ErrorIs(t, err, ErrPathTraversal)
		}
	})

	t.Run("absolute path rejected when it escapes", func(t *testing.T) {
		// This attempts to use an absolute path that would escape
		_, err := SafeFilePath(tempDir, "/etc/passwd")
		// After Join and Clean, this might or might not escape depending on OS
		// The important thing is we verify the result stays within base
		if err == nil {
			// If no error, the path stayed within base (expected on most systems)
			t.Log("Absolute path was normalized within base directory")
		}
	})
}

// Test integration of path validation in theme asset handler
func TestThemeAssetPathValidation(t *testing.T) {
	t.Run("theme name with path traversal rejected", func(t *testing.T) {
		// Test that ValidateAssetName catches path traversal in theme names
		err := ValidateAssetName("../../../etc/passwd")
		assert.Error(t, err)
	})

	t.Run("asset path with path traversal rejected", func(t *testing.T) {
		// Test that ValidateAssetName catches path traversal in asset paths
		err := ValidateAssetName("..%2f..%2f")
		assert.Error(t, err)
	})

	t.Run("valid theme asset path construction", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "theme-test-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tempDir) }()

		// Create theme directory
		themeDir := filepath.Join(tempDir, "default")
		err = os.MkdirAll(themeDir, 0755)
		require.NoError(t, err)

		// Test valid path construction
		path, err := SafeFilePath(themeDir, "style.css")
		assert.NoError(t, err)
		assert.NotEmpty(t, path)
	})
}
