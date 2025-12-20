package timeprovider

import (
	"time"

	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// RealTimeProvider implements TimeProvider using standard time package.
type RealTimeProvider struct{}

// NewRealTimeProvider creates a new real time provider implementation
func NewRealTimeProvider() ports.TimeProvider {
	return &RealTimeProvider{}
}

// Now returns the current time
func (tp *RealTimeProvider) Now() time.Time {
	return time.Now()
}

// Since returns the time elapsed since t
func (tp *RealTimeProvider) Since(t time.Time) time.Duration {
	return time.Since(t)
}

// Until returns the duration until t
func (tp *RealTimeProvider) Until(t time.Time) time.Duration {
	return time.Until(t)
}

// Sleep pauses execution for the given duration
func (tp *RealTimeProvider) Sleep(d time.Duration) {
	time.Sleep(d)
}

// After returns a channel that delivers the current time after d
func (tp *RealTimeProvider) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

// NewTicker creates a new ticker
func (tp *RealTimeProvider) NewTicker(d time.Duration) ports.Ticker {
	return &realTicker{ticker: time.NewTicker(d)}
}

// NewTimer creates a new timer
func (tp *RealTimeProvider) NewTimer(d time.Duration) ports.Timer {
	return &realTimer{timer: time.NewTimer(d)}
}

// realTicker implements Ticker using time.Ticker
type realTicker struct {
	ticker *time.Ticker
}

func (t *realTicker) C() <-chan time.Time {
	return t.ticker.C
}

func (t *realTicker) Stop() {
	t.ticker.Stop()
}

func (t *realTicker) Reset(d time.Duration) {
	t.ticker.Reset(d)
}

// realTimer implements Timer using time.Timer
type realTimer struct {
	timer *time.Timer
}

func (t *realTimer) C() <-chan time.Time {
	return t.timer.C
}

func (t *realTimer) Stop() bool {
	return t.timer.Stop()
}

func (t *realTimer) Reset(d time.Duration) bool {
	return t.timer.Reset(d)
}

// Compile-time interface verification
var _ ports.TimeProvider = (*RealTimeProvider)(nil)
var _ ports.Ticker = (*realTicker)(nil)
var _ ports.Timer = (*realTimer)(nil)
