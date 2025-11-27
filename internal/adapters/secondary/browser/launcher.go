package browser

import (
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"

	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// ErrInvalidBrowserURL is returned when a URL is not safe for browser launch
var ErrInvalidBrowserURL = errors.New("invalid URL for browser launch")

// shellMetachars contains characters that could be used for shell command injection
// on Windows when using cmd.exe
const shellMetachars = "&|;$`!><^()"

// ValidateBrowserURL validates that a URL is safe to pass to browser launch commands.
// It checks for:
// - Valid URL format with http/https scheme only
// - No shell metacharacters that could enable command injection
func ValidateBrowserURL(urlStr string) error {
	// Parse the URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("%w: malformed URL: %v", ErrInvalidBrowserURL, err)
	}

	// Only allow http and https schemes
	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("%w: only http and https schemes are allowed, got: %s", ErrInvalidBrowserURL, scheme)
	}

	// Check hostname for shell metacharacters
	hostname := parsedURL.Hostname()
	if hostname == "" {
		return fmt.Errorf("%w: empty hostname", ErrInvalidBrowserURL)
	}

	// Check entire URL string for shell metacharacters
	for _, char := range shellMetachars {
		if strings.ContainsRune(urlStr, char) {
			return fmt.Errorf("%w: URL contains forbidden character: %c", ErrInvalidBrowserURL, char)
		}
	}

	return nil
}

// Launcher implements the BrowserLauncher interface
type Launcher struct {
	browsers []Browser
}

// Browser represents a browser configuration
type Browser struct {
	Name    string
	Command string
	Args    func(url string) []string
}

// NewLauncher creates a new browser launcher
func NewLauncher() *Launcher {
	return &Launcher{
		browsers: detectBrowsers(),
	}
}

// Launch opens a URL in the default browser
func (l *Launcher) Launch(urlStr string, noOpen bool) error {
	if noOpen {
		return nil
	}

	// Validate URL before launching to prevent command injection
	if err := ValidateBrowserURL(urlStr); err != nil {
		return fmt.Errorf("URL validation failed: %w", err)
	}

	browser, err := l.selectBrowser()
	if err != nil {
		return fmt.Errorf("browser selection: %w", err)
	}

	args := browser.Args(urlStr)
	cmd := exec.Command(browser.Command, args...) // #nosec G204 - URL validated above, browser command validated by selectBrowser

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launching browser: %w", err)
	}

	// Don't wait for browser to close
	go func() {
		_ = cmd.Wait()
	}()

	return nil
}

// Detect detects available browsers
func (l *Launcher) Detect() (string, error) {
	browser, err := l.selectBrowser()
	if err != nil {
		return "", err
	}
	return browser.Name, nil
}

// selectBrowser selects the best available browser
func (l *Launcher) selectBrowser() (*Browser, error) {
	if len(l.browsers) == 0 {
		return nil, errors.New("no browsers available")
	}

	// Return the first browser whose executable is available in PATH.
	for _, candidate := range l.browsers {
		if _, err := exec.LookPath(candidate.Command); err == nil {
			return &candidate, nil
		}
	}

	return nil, errors.New("no supported browsers found on this system")
}

// detectBrowsers detects available browsers based on the platform
func detectBrowsers() []Browser {
	switch runtime.GOOS {
	case "darwin":
		return []Browser{
			{
				Name:    "Chrome",
				Command: "open",
				Args: func(url string) []string {
					return []string{"-a", "Google Chrome", url}
				},
			},
			{
				Name:    "Safari",
				Command: "open",
				Args: func(url string) []string {
					return []string{"-a", "Safari", url}
				},
			},
			{
				Name:    "Firefox",
				Command: "open",
				Args: func(url string) []string {
					return []string{"-a", "Firefox", url}
				},
			},
			{
				Name:    "Default",
				Command: "open",
				Args: func(url string) []string {
					return []string{url}
				},
			},
		}
	case "linux":
		return []Browser{
			{
				Name:    "xdg-open",
				Command: "xdg-open",
				Args: func(url string) []string {
					return []string{url}
				},
			},
			{
				Name:    "Chrome",
				Command: "google-chrome",
				Args: func(url string) []string {
					return []string{url}
				},
			},
			{
				Name:    "Firefox",
				Command: "firefox",
				Args: func(url string) []string {
					return []string{url}
				},
			},
		}
	case "windows":
		// Use rundll32 url.dll,FileProtocolHandler instead of cmd /c start
		// to avoid shell command injection vulnerabilities
		return []Browser{
			{
				Name:    "Default",
				Command: "rundll32",
				Args: func(url string) []string {
					return []string{"url.dll,FileProtocolHandler", url}
				},
			},
			{
				Name:    "Chrome",
				Command: "chrome",
				Args: func(url string) []string {
					return []string{url}
				},
			},
			{
				Name:    "Edge",
				Command: "msedge",
				Args: func(url string) []string {
					return []string{url}
				},
			},
		}
	default:
		return []Browser{}
	}
}

// Ensure Launcher implements ports.BrowserLauncher
var _ ports.BrowserLauncher = (*Launcher)(nil)
