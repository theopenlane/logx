package logx_test

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/theopenlane/logx"
)

func TestSlogLogger(t *testing.T) {
	t.Run("should write slog records through the zerolog logger", func(t *testing.T) {
		b := &bytes.Buffer{}
		l := logx.SlogLogger(zerolog.New(b))

		l.Info("hello", "key", "value")

		assert.Contains(t, b.String(), `"message":"hello"`)
		assert.Contains(t, b.String(), `"key":"value"`)
	})

	t.Run("should respect the underlying zerolog level", func(t *testing.T) {
		b := &bytes.Buffer{}
		l := logx.SlogLogger(zerolog.New(b).Level(zerolog.WarnLevel))

		l.Info("filtered")

		assert.Empty(t, b.String())
	})
}
