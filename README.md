# logx

Opinionated [zerolog](https://github.com/rs/zerolog) wrapper providing context-first structured
logging, request logging middleware for echo, and consistent field naming across services. Extracted
from `core/pkg/logx` so any repo can consume the same logging conventions without importing core.

```
go get github.com/theopenlane/logx
```

## The model: loggers travel on the context

The package is built around one rule: **log through the context, not through a global logger**. A
request-scoped (or job-scoped) logger is attached to the `context.Context` once — by the HTTP
middleware, a job dispatcher, or your own setup code — and every log line downstream retrieves it
from there. Fields attached along the way (request ID, organization, subject) then appear on every
subsequent log line automatically, which is what makes production logs correlatable.

```go
logx.FromContext(ctx).Info().Str("standard_id", id).Msg("standard updated")
```

`FromContext` never returns nil: if the context carries no logger (or a disabled one) it falls back
to zerolog's global logger, so it is always safe to call. This is the method you will use most —
prefer it over `zerolog.Ctx`, `log.Ctx`, or the global `log.Info()` in any code that has a context.

Two things ride on the context and it helps to keep them distinct:

- **The logger itself** — a `zerolog.Logger` whose accumulated fields appear on every event it logs.
- **Durable fields** (`LogFields`) — a plain `map[string]any` mirror of those fields that can be
  read back with `FieldsFromContext`. This exists so field state can cross process boundaries: a job
  queue can serialize the fields when a task is enqueued and restore them with `WithFields` on the
  worker side, keeping correlation intact across the hop.

## Everyday usage

### Adding fields

```go
// one field
ctx = logx.WithField(ctx, logx.FieldOrganizationID, orgID)

// several at once
ctx = logx.WithFields(ctx, map[string]any{
	logx.FieldSubjectID: caller.SubjectID,
	logx.FieldOperation: "standard_update",
})
```

Both update the context logger *and* the durable field store, and both copy the underlying map so
sibling contexts never share state. Use these for fields that should appear on every subsequent log
line in the request or job; use plain zerolog chaining (`.Str(...)`, `.Int(...)`) for fields that
belong to a single event.

### Canonical field names

Use the exported constants instead of string literals when a concept has one — this is what keeps
`organization_id` from also appearing as `org_id`, `owner_id`, and `org` across services:

`FieldRequestID`, `FieldOrganizationID`, `FieldSubjectID`, `FieldSubjectEmail`,
`FieldCapabilities`, `FieldOperation`, `FieldRemoteIP`, `FieldUserAgent`, `FieldRequestProtocol`,
`FieldTrueClientIP`, `FieldForwardedFor`, `FieldRealIP`

### Errors

Always attach errors with `.Err(err)` (never `.Str("error", err.Error())`) so error marshaling and
stack traces work. For picking the *level*, the package classifies expected request-lifecycle
failures — client disconnects and canceled work are not server faults and should not page anyone:

```go
// logs at warn for benign errors (context.Canceled, closed/reset connections,
// sql.ErrTxDone, http.ErrAbortHandler), error for everything else
logx.ErrorEvent(ctx, err).Msg("rollback failed")

// same idea when an HTTP status is in hand: benign errors and 4xx land at warn
logx.FromContext(ctx).WithLevel(logx.RequestErrorLevel(err, status)).Err(err).Msg("request failed")

// the predicate, for your own branching
if logx.BenignError(err) { ... }
```

### Seeding and bridging

```go
// guarantee a context carries a logger (start of a job, background goroutine, etc.)
ctx = logx.SeedContext(ctx)

// hand a *slog.Logger to code that requires the standard library interface (e.g. echo v5's e.Logger);
// events flow through zerolog, which keeps level filtering and output unified
e.Logger = logx.SlogLogger(logger.Unwrap())
```

## Setup at service startup

Call `Configure` once in `main`/server bootstrap; everything else flows from the context.

```go
loggers := logx.Configure(logx.LoggerConfig{
	Level:         zerolog.InfoLevel, // zerolog.NoLevel keeps the existing global default
	Writer:        os.Stderr,         // defaults to os.Stdout
	Pretty:        cfg.PrettyLog,     // human-readable console output for local dev
	IncludeCaller: cfg.Debug,         // file:line on every event; costs a runtime.Caller per event
	IncludeStack:  true,              // render error stack traces (wires zerolog's pkgerrors marshaler)
	WithEcho:      true,              // also build the echo-compatible logger (loggers.Echo)
	SetGlobal:     true,              // make this the zerolog global, so FromContext fallback matches
})
```

Set `SetGlobal` in binaries (servers, CLIs, workers) so code paths without a context-carried logger
still produce consistently formatted output. Leave it unset in tests and libraries.

Every configured logger stamps a `severity` field mapped for GCP Cloud Logging, so log-based metrics
and alerting work without a fluentd rewrite: trace/debug → `DEBUG`, info → `INFO`, warn → `WARNING`,
error → `ERROR`, fatal → `CRITICAL`, panic → `ALERT`.

## Request logging middleware

Both middlewares do the same job: seed the request context with a request-scoped logger (so every
handler and downstream call logs with correlation fields attached), then emit one request-summary
event when the request completes, leveled via `RequestErrorLevel`.

### echo v5 (github.com/labstack/echo/v5)

```go
e.Use(logx.Middleware(logx.MiddlewareConfig{
	Logger:                logger,      // defaults to a stdout logger if nil
	RequestIDHeader:       "X-Request-ID", // header to read the id from (default X-Request-Id)
	RequestIDKey:          logx.FieldRequestID, // log key for the id (default request_id)
	HandleError:           true,
	AttachRequestMetadata: true,
}))
```

Place it after your request-ID middleware and before recovery, so panics are logged with the
request-scoped logger.

### echox (github.com/theopenlane/echox)

```go
e.Use(middleware.RequestID())
e.Use(logx.LoggingMiddleware(logx.Config{
	Logger:                logger,
	HandleError:           true,
	AttachRequestMetadata: true,
}))
```

### Behavior worth knowing

- **`AttachRequestMetadata: true`** puts client origin metadata (`remote_ip`, `user_agent`,
  `request_protocol`, and the `True-Client-IP` / `X-Forwarded-For` / `X-Real-IP` headers when
  present) on the request-scoped logger and durable fields, so *every* log line in the request
  carries them. When unset, the metadata appears only on the final request-summary event.
- **`HandleError: true`** forwards handler errors to the registered `HTTPErrorHandler` and then
  *consumes* them (returns nil), preventing the router from invoking the error handler a second
  time. Leave it false only if another middleware up the chain owns error handling.
- The request-summary event includes `host`, `method`, `uri`, `status`, `latency_ms`, `bytes_in`,
  `bytes_out`, `query`, and `referer`; failed requests are leveled by `RequestErrorLevel`, so
  benign disconnects and 4xx responses log at warn rather than error.
- `Skipper` short-circuits logging for matching requests (health checks, probes).

## Do / don't

- **Do** use `logx.FromContext(ctx)` everywhere a context exists; **don't** use the zerolog global
  or `log.Ctx` in request/job paths.
- **Do** keep log statements on a single line; **don't** split builder chains across newlines.
- **Do** use `.Err(err)` and `ErrorEvent`; **don't** stringify errors into ad hoc fields.
- **Do** use the `Field*` constants for shared concepts; **don't** invent new spellings of an
  existing key.
- **Do** classify expected failures (via `BenignError`/`ErrorEvent`) instead of logging every
  failure at error level.

## Pretty console output

`Pretty: true` routes output through the `consolelog` subpackage — a colorized, human-ordered
console writer for local development. Production should always use the default JSON output.

## Level conversion

`MatchEchoLevel` converts a gommon/echo log level to its zerolog equivalent, for wiring
echox-configured services:

```go
zlvl, _ := logx.MatchEchoLevel(log.WARN) // zerolog.WarnLevel
```

## Composing outputs

`Configure` accepts any `io.Writer`, so zerolog's writer combinators apply directly — e.g.
`zerolog.MultiLevelWriter` to tee destinations, or a `lumberjack.Logger` for rotated files.
