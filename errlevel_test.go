package logx_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"syscall"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/theopenlane/logx"
)

func TestBenignError(t *testing.T) {
	t.Run("should report expected request-lifecycle errors as benign", func(t *testing.T) {
		assert.True(t, logx.BenignError(context.Canceled))
		assert.True(t, logx.BenignError(fmt.Errorf("wrapped: %w", context.Canceled)))
		assert.True(t, logx.BenignError(syscall.ECONNRESET))
		assert.True(t, logx.BenignError(http.ErrAbortHandler))
	})

	t.Run("should report server faults as not benign", func(t *testing.T) {
		assert.False(t, logx.BenignError(errors.New("boom")))
		assert.False(t, logx.BenignError(nil))
	})
}

func TestRequestErrorLevel(t *testing.T) {
	t.Run("should return warn level for benign errors", func(t *testing.T) {
		assert.Equal(t, zerolog.WarnLevel, logx.RequestErrorLevel(context.Canceled, http.StatusInternalServerError))
	})

	t.Run("should return warn level for client error statuses", func(t *testing.T) {
		assert.Equal(t, zerolog.WarnLevel, logx.RequestErrorLevel(errors.New("boom"), http.StatusNotFound))
	})

	t.Run("should return error level for server faults", func(t *testing.T) {
		assert.Equal(t, zerolog.ErrorLevel, logx.RequestErrorLevel(errors.New("boom"), http.StatusInternalServerError))
	})
}

func TestErrorEvent(t *testing.T) {
	t.Run("should log benign errors at warn level", func(t *testing.T) {
		b := &bytes.Buffer{}
		ctx := zerolog.New(b).WithContext(context.Background())

		logx.ErrorEvent(ctx, context.Canceled).Msg("request aborted")

		assert.Contains(t, b.String(), `"level":"warn"`)
		assert.Contains(t, b.String(), context.Canceled.Error())
	})

	t.Run("should log server faults at error level", func(t *testing.T) {
		b := &bytes.Buffer{}
		ctx := zerolog.New(b).WithContext(context.Background())

		logx.ErrorEvent(ctx, errors.New("boom")).Msg("request failed")

		assert.Contains(t, b.String(), `"level":"error"`)
		assert.Contains(t, b.String(), "boom")
	})
}
