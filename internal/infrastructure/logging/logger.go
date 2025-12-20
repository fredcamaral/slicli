// Package logging provides structured logging for the slicli application.
package logging

import (
	"log"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// Logger provides structured logging with level filtering.
type Logger struct {
	component     string
	verbose       bool
	level         entities.LogLevel
	alwaysLogInfo bool // If true, Info/Success log without requiring verbose=true
}

// Option is a function that configures a Logger.
type Option func(*Logger)

// WithVerbose sets the verbose flag.
func WithVerbose(verbose bool) Option {
	return func(l *Logger) {
		l.verbose = verbose
	}
}

// WithLevel sets the log level.
func WithLevel(level entities.LogLevel) Option {
	return func(l *Logger) {
		l.level = level
	}
}

// WithAlwaysLogInfo configures the logger to always log Info/Success without
// requiring verbose=true. This is useful for HTTP server logging.
func WithAlwaysLogInfo(always bool) Option {
	return func(l *Logger) {
		l.alwaysLogInfo = always
	}
}

// NewLogger creates a new logger instance with options.
func NewLogger(component string, opts ...Option) *Logger {
	l := &Logger{
		component:     component,
		verbose:       false,
		level:         entities.LogLevelInfo,
		alwaysLogInfo: false,
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// levelPriority returns the numeric priority for a log level.
func levelPriority(level entities.LogLevel) int {
	priorities := map[entities.LogLevel]int{
		entities.LogLevelDebug: 0,
		entities.LogLevelInfo:  1,
		entities.LogLevelWarn:  2,
		entities.LogLevelError: 3,
	}
	return priorities[level]
}

// shouldLog checks if the message should be logged based on level.
func (l *Logger) shouldLog(msgLevel entities.LogLevel) bool {
	return levelPriority(msgLevel) >= levelPriority(l.level)
}

// formatMessage formats the log message with component prefix if set.
func (l *Logger) formatMessage(level, msg string, args ...interface{}) {
	if l.component != "" {
		log.Printf("[%s] [%s] "+msg, append([]interface{}{level, l.component}, args...)...)
	} else {
		log.Printf("[%s] "+msg, append([]interface{}{level}, args...)...)
	}
}

// Debug logs debug messages (only if debug level is enabled).
func (l *Logger) Debug(msg string, args ...interface{}) {
	if l.shouldLog(entities.LogLevelDebug) {
		l.formatMessage("DEBUG", msg, args...)
	}
}

// Info logs informational messages.
// If alwaysLogInfo is false, requires verbose=true to log.
func (l *Logger) Info(msg string, args ...interface{}) {
	if l.shouldLog(entities.LogLevelInfo) && (l.alwaysLogInfo || l.verbose) {
		l.formatMessage("INFO", msg, args...)
	}
}

// Warn logs warning messages (only if warn level or higher is enabled).
func (l *Logger) Warn(msg string, args ...interface{}) {
	if l.shouldLog(entities.LogLevelWarn) {
		l.formatMessage("WARN", msg, args...)
	}
}

// Error logs error messages (always logged if error level is enabled).
func (l *Logger) Error(msg string, args ...interface{}) {
	if l.shouldLog(entities.LogLevelError) {
		l.formatMessage("ERROR", msg, args...)
	}
}

// Success logs success messages.
// If alwaysLogInfo is false, requires verbose=true to log.
func (l *Logger) Success(msg string, args ...interface{}) {
	if l.shouldLog(entities.LogLevelInfo) && (l.alwaysLogInfo || l.verbose) {
		l.formatMessage("SUCCESS", msg, args...)
	}
}

// SetLevel updates the logging level.
func (l *Logger) SetLevel(level entities.LogLevel) {
	l.level = level
}

// SetVerbose updates the verbose setting.
func (l *Logger) SetVerbose(verbose bool) {
	l.verbose = verbose
}

// GetLevel returns the current logging level.
func (l *Logger) GetLevel() entities.LogLevel {
	return l.level
}

// IsVerbose returns the current verbose setting.
func (l *Logger) IsVerbose() bool {
	return l.verbose
}
