package logx

import (
	"log/slog"

	"github.com/rs/zerolog"
)

// SlogLogger wraps a zerolog logger in a *slog.Logger for consumers that require the standard library interface;
// level filtering is left to the underlying zerolog logger
func SlogLogger(l zerolog.Logger) *slog.Logger {
	return slog.New(zerolog.NewSlogHandler(l))
}
