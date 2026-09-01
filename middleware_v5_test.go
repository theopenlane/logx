package logx_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	echo "github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"

	"github.com/theopenlane/logx"
)

func newV5Logger(b *bytes.Buffer) *logx.Logger {
	return logx.Configure(logx.LoggerConfig{
		Writer:   b,
		WithEcho: true,
	}).Echo
}

func TestV5Middleware(t *testing.T) {
	t.Run("should log the request and seed the context logger", func(t *testing.T) {
		b := &bytes.Buffer{}
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		m := logx.Middleware(logx.MiddlewareConfig{Logger: newV5Logger(b)})

		var seeded bool

		next := func(c *echo.Context) error {
			seeded = logx.FromContext(c.Request().Context()) != nil

			return nil
		}

		err := m(next)(c)

		assert.NoError(t, err)
		assert.True(t, seeded, "should seed the request context with a logger")
		assert.Contains(t, b.String(), `"method":"GET"`)
	})

	t.Run("should add the request id to log records and durable fields", func(t *testing.T) {
		b := &bytes.Buffer{}
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Request-ID", "req-123")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		m := logx.Middleware(logx.MiddlewareConfig{Logger: newV5Logger(b)})

		var fields logx.LogFields

		next := func(c *echo.Context) error {
			fields = logx.FieldsFromContext(c.Request().Context())

			return nil
		}

		err := m(next)(c)

		assert.NoError(t, err)
		assert.Equal(t, "req-123", fields["request_id"])
	})

	t.Run("should attach request metadata when configured", func(t *testing.T) {
		b := &bytes.Buffer{}
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("User-Agent", "test-agent")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		m := logx.Middleware(logx.MiddlewareConfig{
			Logger:                newV5Logger(b),
			AttachRequestMetadata: true,
		})

		var fields logx.LogFields

		next := func(c *echo.Context) error {
			fields = logx.FieldsFromContext(c.Request().Context())

			return nil
		}

		err := m(next)(c)

		assert.NoError(t, err)
		assert.Equal(t, "test-agent", fields["user_agent"])
	})

	t.Run("should skip the middleware when the skipper returns true", func(t *testing.T) {
		b := &bytes.Buffer{}
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		m := logx.Middleware(logx.MiddlewareConfig{
			Logger:  newV5Logger(b),
			Skipper: func(_ *echo.Context) bool { return true },
		})

		next := func(c *echo.Context) error {
			return nil
		}

		err := m(next)(c)

		assert.NoError(t, err)
		assert.Empty(t, b.String(), "should not log skipped requests")
	})

	t.Run("should not trigger error handler when HandleError is false", func(t *testing.T) {
		b := &bytes.Buffer{}
		var called bool

		e := echo.New()
		e.HTTPErrorHandler = func(c *echo.Context, err error) {
			called = true
		}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		m := logx.Middleware(logx.MiddlewareConfig{Logger: newV5Logger(b)})

		next := func(c *echo.Context) error {
			return errors.New("boom")
		}

		err := m(next)(c)

		assert.Error(t, err, "should return error")
		assert.False(t, called, "should not call error handler")
	})

	t.Run("should trigger error handler when HandleError is true", func(t *testing.T) {
		b := &bytes.Buffer{}
		var called bool

		e := echo.New()
		e.HTTPErrorHandler = func(c *echo.Context, err error) {
			called = true

			c.JSON(http.StatusInternalServerError, err.Error())
		}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		m := logx.Middleware(logx.MiddlewareConfig{
			Logger:      newV5Logger(b),
			HandleError: true,
		})

		next := func(c *echo.Context) error {
			return errors.New("boom")
		}

		err := m(next)(c)

		assert.NoError(t, err, "should not return error once the error handler has been invoked")
		assert.True(t, called, "should call error handler")
		assert.Contains(t, b.String(), `"error":"boom"`)
	})
}
