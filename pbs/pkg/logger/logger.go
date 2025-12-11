// Package logger provides structured logging for the PBS server
package logger

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// ContextKey is the type for context keys
type ContextKey string

const (
	// RequestIDKey is the context key for request IDs
	RequestIDKey ContextKey = "request_id"
	// AuctionIDKey is the context key for auction IDs
	AuctionIDKey ContextKey = "auction_id"
)

var (
	// Log is the global logger instance
	Log zerolog.Logger
)

// Config holds logger configuration
type Config struct {
	Level      string // debug, info, warn, error
	Format     string // json, console
	TimeFormat string // time format for console output
}

// DefaultConfig returns sensible defaults for production
func DefaultConfig() Config {
	return Config{
		Level:      getEnv("LOG_LEVEL", "info"),
		Format:     getEnv("LOG_FORMAT", "json"),
		TimeFormat: time.RFC3339,
	}
}

// Init initializes the global logger
func Init(cfg Config) {
	var output io.Writer = os.Stdout

	// Parse log level
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}

	// Configure output format
	if cfg.Format == "console" {
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: cfg.TimeFormat,
		}
	}

	// Create logger with common fields
	Log = zerolog.New(output).
		Level(level).
		With().
		Timestamp().
		Str("service", "pbs").
		Logger()
}

// WithRequestID adds a request ID to the logger context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// WithAuctionID adds an auction ID to the logger context
func WithAuctionID(ctx context.Context, auctionID string) context.Context {
	return context.WithValue(ctx, AuctionIDKey, auctionID)
}

// FromContext returns a logger with context values
func FromContext(ctx context.Context) zerolog.Logger {
	l := Log.With()

	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		l = l.Str("request_id", requestID)
	}

	if auctionID, ok := ctx.Value(AuctionIDKey).(string); ok {
		l = l.Str("auction_id", auctionID)
	}

	return l.Logger()
}

// Auction returns a logger for auction events
func Auction(auctionID string) zerolog.Logger {
	return Log.With().Str("auction_id", auctionID).Logger()
}

// Bidder returns a logger for bidder events
func Bidder(bidderCode string) zerolog.Logger {
	return Log.With().Str("bidder", bidderCode).Logger()
}

// HTTP returns a logger for HTTP events
func HTTP() zerolog.Logger {
	return Log.With().Str("component", "http").Logger()
}

// IDR returns a logger for IDR client events
func IDR() zerolog.Logger {
	return Log.With().Str("component", "idr").Logger()
}

// getEnv returns environment variable or default
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// RequestLogger holds request-scoped logging state
type RequestLogger struct {
	logger    zerolog.Logger
	startTime time.Time
}

// NewRequestLogger creates a new request-scoped logger
func NewRequestLogger(requestID string) *RequestLogger {
	return &RequestLogger{
		logger:    Log.With().Str("request_id", requestID).Logger(),
		startTime: time.Now(),
	}
}

// Info logs an info message
func (r *RequestLogger) Info(msg string) {
	r.logger.Info().Msg(msg)
}

// Error logs an error message
func (r *RequestLogger) Error(msg string, err error) {
	r.logger.Error().Err(err).Msg(msg)
}

// WithField adds a field to the logger
func (r *RequestLogger) WithField(key string, value interface{}) *RequestLogger {
	r.logger = r.logger.With().Interface(key, value).Logger()
	return r
}

// Duration returns the time since the request started
func (r *RequestLogger) Duration() time.Duration {
	return time.Since(r.startTime)
}

// LogComplete logs request completion with duration
func (r *RequestLogger) LogComplete(status int) {
	r.logger.Info().
		Int("status", status).
		Dur("duration_ms", r.Duration()).
		Msg("request completed")
}
