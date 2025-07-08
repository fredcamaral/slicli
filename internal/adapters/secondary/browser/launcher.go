package browser

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/fredcamaral/slicli/internal/domain/ports"
)

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
func (l *Launcher) Launch(url string, noOpen bool) error {
	if noOpen {
		return nil
	}

	browser, err := l.selectBrowser()
	if err != nil {
		return fmt.Errorf("browser selection: %w", err)
	}

	args := browser.Args(url)
	cmd := exec.Command(browser.Command, args...) // #nosec G204 - browser command validated by selectBrowser

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
		return []Browser{
			{
				Name:    "Default",
				Command: "cmd",
				Args: func(url string) []string {
					return []string{"/c", "start", url}
				},
			},
			{
				Name:    "Chrome",
				Command: "cmd",
				Args: func(url string) []string {
					return []string{"/c", "start", "chrome", url}
				},
			},
			{
				Name:    "Edge",
				Command: "cmd",
				Args: func(url string) []string {
					return []string{"/c", "start", "msedge", url}
				},
			},
		}
	default:
		return []Browser{}
	}
}

// Ensure Launcher implements ports.BrowserLauncher
var _ ports.BrowserLauncher = (*Launcher)(nil)
