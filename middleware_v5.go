package logx

import (
	"os"
	"time"

	echo "github.com/labstack/echo/v5"
)

// MiddlewareConfig configures the echo v5 request logging middleware
type MiddlewareConfig struct {
	// Logger is a custom instance of the logger to use
	Logger *Logger
	// Skipper short-circuits the middleware when it returns true
	Skipper func(c *echo.Context) bool
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

// Middleware returns echo v5 middleware that logs requests using the configured logger and seeds
// the request context with a request-scoped logger carrying durable fields
func Middleware(config MiddlewareConfig) echo.MiddlewareFunc {
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

	// the shared logging helpers key off the legacy config shape
	shared := Config{
		AttachRequestMetadata: config.AttachRequestMetadata,
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if config.Skipper != nil && config.Skipper(c) {
				return next(c)
			}

			req := c.Request()
			start := time.Now()

			logger := config.Logger

			// start with the request context and enrich it with durable fields
			ctx := req.Context()

			id := getRequestID(req, c.Response().Header(), config.RequestIDHeader)
			if id != "" {
				logger = newLoggerFromExisting(logger.log.With().Str(config.RequestIDKey, id).Logger(), logger.out)
				ctx = storeDurableField(ctx, config.RequestIDKey, id)
			}

			if config.AttachRequestMetadata {
				logger, ctx = attachRequestMetadataWithDurableFields(ctx, req, c.RealIP(), logger)
			}

			// the request context is retrieved and set to the logger's context, then installed on the request
			c.SetRequest(req.WithContext(logger.WithContext(ctx)))

			err := next(c)
			if err != nil && config.HandleError {
				c.Echo().HTTPErrorHandler(c, err)
			}

			if res, resErr := echo.UnwrapResponse(c.Response()); resErr == nil {
				logEvent(c.Request(), c.RealIP(), res.Status, res.Size, logger, shared, start, err)
			}

			// once the error handler has been invoked the error is consumed; returning it
			// would trigger the router's error handling a second time
			if config.HandleError {
				return nil
			}

			return err
		}
	}
}
