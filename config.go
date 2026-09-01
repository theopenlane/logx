package logx

import (
	"io"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"

	"github.com/theopenlane/logx/consolelog"
)

// LoggerConfig controls how loggers produced by Configure behave
type LoggerConfig struct {
	// Level sets the zerolog level. Use zerolog.NoLevel to keep the existing global default
	Level zerolog.Level
	// Pretty toggles human-readable console output instead of structured JSON
	Pretty bool
	// Writer controls where logs are written. Defaults to os.Stdout
	Writer io.Writer
	// IncludeCaller attaches caller information to each log entry
	IncludeCaller bool
	// IncludeStack wires zerolog's ErrorStackMarshaler to pkgerrors.MarshalStack so events
	// carrying an error render its stack trace under the stack field
	IncludeStack bool
	// WithEcho instructs Configure to build an echo-compatible logger
	WithEcho bool
	// SetGlobal updates zerolog's global logger (`log.Logger`) using the configured settings
	SetGlobal bool
}

// LoggerSet contains the loggers produced by Configure
type LoggerSet struct {
	// Logger is the configured root zerolog logger
	Logger zerolog.Logger
	// Echo is the echo-compatible logger, populated only when LoggerConfig.WithEcho is set
	Echo *Logger
}

// Configure builds loggers according to the supplied configuration
func Configure(cfg LoggerConfig) LoggerSet {
	// stack marshaling is a zerolog package-level setting and applies to all loggers
	if cfg.IncludeStack {
		zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	}

	root, output := buildRootLogger(cfg)

	if cfg.SetGlobal {
		if cfg.Level != zerolog.NoLevel {
			zerolog.SetGlobalLevel(cfg.Level)
		}

		log.Logger = root
	}

	var echoLogger *Logger
	if cfg.WithEcho {
		echoLogger = newLoggerFromExisting(root, output)
	}

	return LoggerSet{
		Logger: root,
		Echo:   echoLogger,
	}
}

// buildRootLogger constructs the root zerolog logger from the configuration, returning the logger
// and the resolved output writer
func buildRootLogger(cfg LoggerConfig) (zerolog.Logger, io.Writer) {
	writer := cfg.Writer
	if writer == nil {
		writer = os.Stdout
	}

	output := writer

	if cfg.Pretty {
		cw := consolelog.NewConsoleWriter()
		output = &cw
	}

	zctx := zerolog.New(output).With().Timestamp()

	if cfg.IncludeCaller {
		zctx = zctx.Caller()
	}

	root := zctx.Logger().Hook(severityHook)

	if cfg.Level != zerolog.NoLevel {
		root = root.Level(cfg.Level)
	}

	return root, output
}
