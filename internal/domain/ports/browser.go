package ports

// BrowserLauncher defines the interface for launching browsers
type BrowserLauncher interface {
	// Launch opens a URL in the default browser
	Launch(url string, noOpen bool) error
	// Detect detects available browsers
	Detect() (string, error)
}
