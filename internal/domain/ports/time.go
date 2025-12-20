package ports

import "time"

//go:generate mockery --name TimeProvider --output ../../../test/mocks --outpkg mocks

// TimeProvider abstracts time operations for testability
type TimeProvider interface {
	Now() time.Time
	Since(t time.Time) time.Duration
	Until(t time.Time) time.Duration
	Sleep(d time.Duration)
	After(d time.Duration) <-chan time.Time
	NewTicker(d time.Duration) Ticker
	NewTimer(d time.Duration) Timer
}

// Ticker abstracts time.Ticker for testability
type Ticker interface {
	C() <-chan time.Time
	Stop()
	Reset(d time.Duration)
}

// Timer abstracts time.Timer for testability
type Timer interface {
	C() <-chan time.Time
	Stop() bool
	Reset(d time.Duration) bool
}
