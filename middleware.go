package logx

import (
	"context"
	"maps"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog"

	echo "github.com/theopenlane/echox"
	"github.com/theopenlane/echox/middleware"
)

// headerTrueClientIP is the header carrying the client origin IP behind some proxies
const headerTrueClientIP = "True-Client-IP"

// Config defines the config for the echolog middleware
type Config struct {
	// Logger is a custom instance of the logger to use
	Logger *Logger
	// Skipper defines a function to skip middleware
	Skipper middleware.Skipper
	// RequestIDHeader is the header name to use for the request ID in a log record
	RequestIDHeader string
	// RequestIDKey is the key name to use for the request ID in a log record
	RequestIDKey string
	// HandleError indicates whether to forward errors to the global error handler, so it can decide the appropriate status code
	HandleError bool
	// AttachRequestMetadata controls whether stable request metadata (client origin IP, user agent, and forwarding headers)
	// is attached to the request-scoped logger context so downstream log entries can include it
	AttachRequestMetadata bool
}

// loggerContext wraps echo.Context so c.Logger() returns the request-scoped logger
type loggerContext struct {
	echo.Context
	logger *Logger
}

// Logger returns the logger from the context
func (c *loggerContext) Logger() echo.Logger {
	return c.logger
}

// LoggingMiddleware is a middleware that logs requests using the provided logger
func LoggingMiddleware(config Config) echo.MiddlewareFunc {
	if config.Skipper == nil {
		config.Skipper = middleware.DefaultSkipper
	}

	if config.Logger == nil {
		config.Logger = Configure(LoggerConfig{
			Writer:   os.Stdout,
			WithEcho: true,
		}).Echo
	}

	if config.RequestIDKey == "" {
		config.RequestIDKey = FieldRequestID
	}

	if config.RequestIDHeader == "" {
		config.RequestIDHeader = echo.HeaderXRequestID
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}

			var err error

			req := c.Request()
			start := time.Now()

			logger := config.Logger

			// Start with the request context and enrich it with durable fields
			ctx := req.Context()

			id := getRequestID(req, c.Response().Header(), config.RequestIDHeader)
			if id != "" {
				logger = newLoggerFromExisting(logger.log.With().Str(config.RequestIDKey, id).Logger(), logger.out)
				ctx = storeDurableField(ctx, config.RequestIDKey, id)
			}

			if config.AttachRequestMetadata {
				logger, ctx = attachRequestMetadataWithDurableFields(ctx, req, c.RealIP(), logger)
			}

			// The request context is retrieved and set to the logger's context
			// the context is then set to the request, and a new context is created with the logger
			c.SetRequest(req.WithContext(logger.WithContext(ctx)))
			c = &loggerContext{c, logger}

			if err = next(c); err != nil {
				if config.HandleError {
					c.Error(err)
				}
			}

			logEvent(c.Request(), c.RealIP(), c.Response().Status, c.Response().Size, logger, config, start, err)

			if config.HandleError {
				return nil
			}

			return err
		}
	}
}

// getRequestID retrieves the request ID from the request or response headers
func getRequestID(req *http.Request, resHeader http.Header, header string) string {
	id := req.Header.Get(header)
	if id == "" {
		id = resHeader.Get(header)
	}

	return id
}

// logEvent logs the event with all the necessary details; it uses the error and response status to determine the log level
func logEvent(req *http.Request, remoteIP string, status int, sizeBytes int64, logger *Logger, config Config, start time.Time, err error) {
	latency := time.Since(start)

	var evt *zerolog.Event
	// this is the error that's passed in as input from the middleware func

	switch {
	case err != nil:
		evt = logger.log.WithLevel(RequestErrorLevel(err, status)).Stack().Err(err)
	default:
		evt = logger.log.WithLevel(logger.log.GetLevel())
	}

	// Only log request metadata here if it's NOT already in the logger context
	if !config.AttachRequestMetadata {
		evt.Str(FieldRemoteIP, remoteIP)
		evt.Str(FieldUserAgent, req.UserAgent())
		evt.Str(FieldRequestProtocol, req.Proto)

		if trueClientIP := req.Header.Get(headerTrueClientIP); trueClientIP != "" {
			evt.Str(FieldTrueClientIP, trueClientIP)
		}

		if forwardedFor := req.Header.Get(echo.HeaderXForwardedFor); forwardedFor != "" {
			evt.Str(FieldForwardedFor, forwardedFor)
		}

		if realIP := req.Header.Get(echo.HeaderXRealIP); realIP != "" {
			evt.Str(FieldRealIP, realIP)
		}
	}

	evt.Str("host", req.Host)
	evt.Str("method", req.Method)
	evt.Str("uri", req.RequestURI)
	evt.Int("status", status)
	evt.Str("referer", req.Referer())
	evt.Int64("latency_ms", latency.Milliseconds())
	evt.Str("latency_human", latency.String())

	if query := req.URL.RawQuery; query != "" {
		evt.Str("query", query)
	}

	cl := req.Header.Get(echo.HeaderContentLength)
	if cl == "" {
		cl = "0"
	}

	evt.Str("bytes_in", cl)
	evt.Str("bytes_out", strconv.FormatInt(sizeBytes, 10))

	evt.Msgf("request details for request to %s %s", req.Method, req.RequestURI)
}

// attachRequestMetadataWithDurableFields attaches stable request metadata to the logger and stores fields durably on context
func attachRequestMetadataWithDurableFields(ctx context.Context, req *http.Request, remoteIP string, logger *Logger) (*Logger, context.Context) {
	zctx := logger.log.With().
		Str(FieldRemoteIP, remoteIP).
		Str(FieldUserAgent, req.UserAgent()).
		Str(FieldRequestProtocol, req.Proto)

	ctx = storeDurableField(ctx, FieldRemoteIP, remoteIP)
	ctx = storeDurableField(ctx, FieldUserAgent, req.UserAgent())
	ctx = storeDurableField(ctx, FieldRequestProtocol, req.Proto)

	if trueClientIP := req.Header.Get(headerTrueClientIP); trueClientIP != "" {
		zctx = zctx.Str(FieldTrueClientIP, trueClientIP)
		ctx = storeDurableField(ctx, FieldTrueClientIP, trueClientIP)
	}

	if forwardedFor := req.Header.Get(echo.HeaderXForwardedFor); forwardedFor != "" {
		zctx = zctx.Str(FieldForwardedFor, forwardedFor)
		ctx = storeDurableField(ctx, FieldForwardedFor, forwardedFor)
	}

	if realIP := req.Header.Get(echo.HeaderXRealIP); realIP != "" {
		zctx = zctx.Str(FieldRealIP, realIP)
		ctx = storeDurableField(ctx, FieldRealIP, realIP)
	}

	return newLoggerFromExisting(zctx.Logger(), logger.out), ctx
}

// storeDurableField stores a field in the durable log fields on the context;
// a new LogFields map is allocated on each call so that sibling contexts do not share mutable state
func storeDurableField(ctx context.Context, key string, value any) context.Context {
	existing := FieldsFromContext(ctx)
	fields := make(LogFields, len(existing)+1)

	maps.Copy(fields, existing)

	fields[key] = value

	return logFieldsContextKey.Set(ctx, fields)
}
