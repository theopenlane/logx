package logx

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/samber/lo"
)

// benignErrors are the expected request-lifecycle errors
var benignErrors = []error{
	context.Canceled,
	sql.ErrTxDone,
	syscall.EPIPE,
	syscall.ECONNRESET,
	net.ErrClosed,
	http.ErrAbortHandler,
}

// BenignError reports whether err is an expected request-lifecycle failure rather than a server fault
func BenignError(err error) bool {
	return lo.SomeBy(benignErrors, func(target error) bool { return errors.Is(err, target) })
}

// ErrorEvent returns a context-logger event for err at error level, or warn level when the error is benign
func ErrorEvent(ctx context.Context, err error) *zerolog.Event {
	if BenignError(err) {
		return FromContext(ctx).Warn().Err(err)
	}

	return FromContext(ctx).Error().Err(err)
}

// RequestErrorLevel returns the log level for a request that finished with err and HTTP status
func RequestErrorLevel(err error, status int) zerolog.Level {
	if BenignError(err) || (status >= http.StatusBadRequest && status < http.StatusInternalServerError) {
		return zerolog.WarnLevel
	}

	return zerolog.ErrorLevel
}
