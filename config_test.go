package logx_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/theopenlane/logx"
)

func TestConfigureWithCaller(t *testing.T) {
	b := &bytes.Buffer{}

	logger := logx.Configure(logx.LoggerConfig{
		Writer:        b,
		WithEcho:      true,
		IncludeCaller: true,
	}).Echo

	logger.Print("foobar")

	var entry map[string]any
	err := json.Unmarshal(b.Bytes(), &entry)
	assert.NoError(t, err)

	segments := strings.Split(entry["caller"].(string), ":")
	filePath := filepath.Base(segments[0])

	assert.Equal(t, "logger.go", filePath)
}

func TestConfigureWithLevel(t *testing.T) {
	b := &bytes.Buffer{}

	logger := logx.Configure(logx.LoggerConfig{
		Writer:   b,
		WithEcho: true,
		Level:    zerolog.WarnLevel,
	}).Echo

	logger.Debug("Test")
	assert.Equal(t, "", b.String())

	logger.Warn("Foobar")

	var entry map[string]any
	err := json.Unmarshal(b.Bytes(), &entry)
	assert.NoError(t, err)
	assert.Equal(t, "Foobar", entry["message"])
	assert.Equal(t, "warn", entry["level"])
}

func TestConfigureIncludeStack(t *testing.T) {
	prev := zerolog.ErrorStackMarshaler
	zerolog.ErrorStackMarshaler = nil

	t.Cleanup(func() { zerolog.ErrorStackMarshaler = prev })

	logx.Configure(logx.LoggerConfig{
		Writer:       &bytes.Buffer{},
		IncludeStack: true,
	})

	assert.NotNil(t, zerolog.ErrorStackMarshaler, "should wire the stack marshaler")
}

func TestConfigureWithTimestamp(t *testing.T) {
	b := &bytes.Buffer{}

	logger := logx.Configure(logx.LoggerConfig{
		Writer:   b,
		WithEcho: true,
	}).Echo

	logger.Print("foobar")

	var entry struct {
		Level   string    `json:"level"`
		Message string    `json:"message"`
		Time    time.Time `json:"time"`
	}

	err := json.Unmarshal(b.Bytes(), &entry)

	assert.NoError(t, err)
	assert.NotEmpty(t, entry.Time)
	assert.Equal(t, "foobar", entry.Message)
}
