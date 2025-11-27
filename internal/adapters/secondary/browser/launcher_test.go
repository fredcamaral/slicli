package browser

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test ValidateBrowserURL security function
func TestValidateBrowserURL(t *testing.T) {
	t.Run("valid HTTP URL passes", func(t *testing.T) {
		err := ValidateBrowserURL("http://localhost:8080")
		assert.NoError(t, err)
	})

	t.Run("valid HTTPS URL passes", func(t *testing.T) {
		err := ValidateBrowserURL("https://example.com")
		assert.NoError(t, err)
	})

	t.Run("URL with path passes", func(t *testing.T) {
		err := ValidateBrowserURL("http://localhost:8080/slides")
		assert.NoError(t, err)
	})

	t.Run("file scheme rejected", func(t *testing.T) {
		err := ValidateBrowserURL("file:///etc/passwd")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidBrowserURL)
		assert.Contains(t, err.Error(), "only http and https schemes")
	})

	t.Run("javascript scheme rejected", func(t *testing.T) {
		err := ValidateBrowserURL("javascript:alert(1)")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidBrowserURL)
	})

	t.Run("data scheme rejected", func(t *testing.T) {
		err := ValidateBrowserURL("data:text/html,<script>alert(1)</script>")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidBrowserURL)
	})

	t.Run("empty hostname rejected", func(t *testing.T) {
		err := ValidateBrowserURL("http:///path")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidBrowserURL)
		assert.Contains(t, err.Error(), "empty hostname")
	})

	t.Run("shell metacharacter ampersand rejected", func(t *testing.T) {
		err := ValidateBrowserURL("http://example.com&whoami")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidBrowserURL)
		assert.Contains(t, err.Error(), "forbidden character")
	})

	t.Run("shell metacharacter pipe rejected", func(t *testing.T) {
		err := ValidateBrowserURL("http://example.com|whoami")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidBrowserURL)
	})

	t.Run("shell metacharacter semicolon rejected", func(t *testing.T) {
		err := ValidateBrowserURL("http://example.com;whoami")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidBrowserURL)
	})

	t.Run("shell metacharacter dollar rejected", func(t *testing.T) {
		err := ValidateBrowserURL("http://example.com$USER")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidBrowserURL)
	})

	t.Run("shell metacharacter backtick rejected", func(t *testing.T) {
		err := ValidateBrowserURL("http://example.com`whoami`")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidBrowserURL)
	})

	t.Run("shell metacharacter exclamation rejected", func(t *testing.T) {
		err := ValidateBrowserURL("http://example.com!cmd")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidBrowserURL)
	})

	t.Run("shell metacharacter redirect rejected", func(t *testing.T) {
		err := ValidateBrowserURL("http://example.com>file")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidBrowserURL)
	})

	t.Run("shell metacharacter caret rejected", func(t *testing.T) {
		err := ValidateBrowserURL("http://example.com^cmd")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidBrowserURL)
	})

	t.Run("shell metacharacter parenthesis rejected", func(t *testing.T) {
		err := ValidateBrowserURL("http://example.com()")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidBrowserURL)
	})

	t.Run("malformed URL rejected", func(t *testing.T) {
		err := ValidateBrowserURL("://invalid")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidBrowserURL)
	})
}

func TestLaunchWithURLValidation(t *testing.T) {
	t.Run("malicious URL blocked before browser launch", func(t *testing.T) {
		launcher := NewLauncher()
		err := launcher.Launch("http://example.com&whoami", false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "URL validation failed")
	})

	t.Run("javascript URL blocked", func(t *testing.T) {
		launcher := NewLauncher()
		err := launcher.Launch("javascript:alert(document.cookie)", false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "URL validation failed")
	})
}

func TestNewLauncher(t *testing.T) {
	launcher := NewLauncher()
	assert.NotNil(t, launcher)
	assert.NotEmpty(t, launcher.browsers)
}

func TestLauncherLaunch(t *testing.T) {
	t.Run("with noOpen flag", func(t *testing.T) {
		launcher := NewLauncher()
		err := launcher.Launch("http://localhost:8080", true)
		assert.NoError(t, err)
	})

	t.Run("without browsers", func(t *testing.T) {
		launcher := &Launcher{browsers: []Browser{}}
		err := launcher.Launch("http://localhost:8080", false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "browser selection")
	})

	// Note: We can't easily test actual browser launching in unit tests
	// as it would open a browser window. This would be tested manually.
}

func TestLauncherDetect(t *testing.T) {
	t.Run("with browsers available", func(t *testing.T) {
		launcher := NewLauncher()
		name, err := launcher.Detect()
		assert.NoError(t, err)
		assert.NotEmpty(t, name)
	})

	t.Run("without browsers", func(t *testing.T) {
		launcher := &Launcher{browsers: []Browser{}}
		_, err := launcher.Detect()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no browsers available")
	})
}

func TestSelectBrowser(t *testing.T) {
	t.Run("with browsers available", func(t *testing.T) {
		launcher := &Launcher{
			browsers: []Browser{
				{Name: "TestBrowser", Command: "test", Args: func(url string) []string { return []string{url} }},
			},
		}
		browser, err := launcher.selectBrowser()
		require.NoError(t, err)
		assert.Equal(t, "TestBrowser", browser.Name)
	})

	t.Run("without browsers", func(t *testing.T) {
		launcher := &Launcher{browsers: []Browser{}}
		_, err := launcher.selectBrowser()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no browsers available")
	})
}

func TestDetectBrowsers(t *testing.T) {
	browsers := detectBrowsers()

	switch runtime.GOOS {
	case "darwin":
		assert.NotEmpty(t, browsers)
		// Check for expected browsers on macOS
		names := make(map[string]bool)
		for _, b := range browsers {
			names[b.Name] = true
		}
		assert.True(t, names["Chrome"])
		assert.True(t, names["Safari"])
		assert.True(t, names["Default"])
	case "linux":
		assert.NotEmpty(t, browsers)
		// Check for expected browsers on Linux
		names := make(map[string]bool)
		for _, b := range browsers {
			names[b.Name] = true
		}
		assert.True(t, names["xdg-open"])
	case "windows":
		assert.NotEmpty(t, browsers)
		// Check for expected browsers on Windows
		names := make(map[string]bool)
		for _, b := range browsers {
			names[b.Name] = true
		}
		assert.True(t, names["Default"])
	default:
		// Unknown platform should return empty
		assert.Empty(t, browsers)
	}
}

func TestBrowserArgs(t *testing.T) {
	testURL := "http://localhost:8080"

	t.Run("macOS browsers", func(t *testing.T) {
		if runtime.GOOS != "darwin" {
			t.Skip("macOS-specific test")
		}

		browsers := detectBrowsers()
		for _, browser := range browsers {
			args := browser.Args(testURL)
			assert.NotEmpty(t, args)
			// URL should be in the args
			assert.Contains(t, args, testURL)
		}
	})

	t.Run("Linux browsers", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("Linux-specific test")
		}

		browsers := detectBrowsers()
		for _, browser := range browsers {
			args := browser.Args(testURL)
			assert.NotEmpty(t, args)
			assert.Contains(t, args, testURL)
		}
	})

	t.Run("Windows browsers", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("Windows-specific test")
		}

		browsers := detectBrowsers()
		for _, browser := range browsers {
			args := browser.Args(testURL)
			assert.NotEmpty(t, args)
			// On Windows, URL might be in different positions
			argsStr := ""
			for _, arg := range args {
				argsStr += arg + " "
			}
			assert.Contains(t, argsStr, testURL)
		}
	})
}
