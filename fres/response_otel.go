package fres

import (
	"net/http"
	"sync/atomic"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// otelEnabled controls whether fres records OpenTelemetry spans at the response
// exit. Enabled by default; disable globally via SetOtelEnabled(false).
var otelEnabled atomic.Bool

func init() {
	otelEnabled.Store(true)
}

// SetOtelEnabled toggles OpenTelemetry recording for the fres package globally.
// When disabled, ResponseOtel / ErrJsonOtel fall back to plain responses with no
// tracing overhead. Per-call options (WithOtel / WithoutOtel) still take
// precedence over this switch.
func SetOtelEnabled(enabled bool) {
	otelEnabled.Store(enabled)
}

// IsOtelEnabled reports the current global OpenTelemetry switch.
func IsOtelEnabled() bool {
	return otelEnabled.Load()
}

// OtelOption fine-tunes OpenTelemetry behavior for a single response.
type OtelOption func(*otelConfig)

type otelConfig struct {
	enabled bool
}

// WithoutOtel disables tracing for this single response (overrides the global
// switch). Handy for noisy or sensitive endpoints you don't want traced.
func WithoutOtel() OtelOption {
	return func(c *otelConfig) { c.enabled = false }
}

// WithOtel forces tracing for this single response even when globally disabled.
func WithOtel() OtelOption {
	return func(c *otelConfig) { c.enabled = true }
}

func applyOtelOpts(opts []OtelOption) otelConfig {
	cfg := otelConfig{enabled: otelEnabled.Load()}
	for _, o := range opts {
		o(&cfg)
	}
	return cfg
}

// ResponseOtel is the OpenTelemetry-aware variant of Response.
//
// On err != nil it records the business error info (biz.code, biz.err_msg) and
// the underlying error onto the current span, then writes the error response.
// On success it behaves exactly like Response.
//
// Example:
//
//	resp, err := logic.Do(req)
//	fres.ResponseOtel(c, resp, err)
//
// Per-call override:
//
//	fres.ResponseOtel(c, resp, err, fres.WithoutOtel()) // skip tracing this once
func ResponseOtel(c *gin.Context, response interface{}, err error, opts ...OtelOption) {
	if err == nil {
		OkJson(c, response)
		return
	}
	if applyOtelOpts(opts).enabled {
		recordErrToSpan(c, response, err)
	}
	ErrJson(c, response)
}

// ErrJsonOtel is the OpenTelemetry-aware variant of ErrJson.
//
// It records the error onto the current span and then writes the error response.
// Use it for paths that return an error directly without going through Response
// (e.g. parameter validation). err may be nil — only biz.err_msg is recorded then.
func ErrJsonOtel(c *gin.Context, response interface{}, err error, opts ...OtelOption) {
	if applyOtelOpts(opts).enabled {
		recordErrToSpan(c, response, err)
	}
	ErrJson(c, response)
}

// recordErrToSpan records the business error info and the underlying error onto
// the OpenTelemetry span carried by the request context. No-op when there is no
// recording span (e.g. tracing not initialized, or otel disabled upstream), so
// callers pay nothing in those cases.
func recordErrToSpan(c *gin.Context, res interface{}, err error) {
	span := trace.SpanFromContext(c.Request.Context())
	if !span.IsRecording() {
		return
	}

	httpCode := http.StatusInternalServerError
	if e, ok := res.(ResponseErr); ok {
		if e.Type == 0 {
			httpCode = http.StatusBadRequest
		}
		span.SetAttributes(
			attribute.Int("biz.code", e.Code),
			attribute.String("biz.err_msg", e.ErrMsg),
		)
	}
	span.SetAttributes(attribute.Int("http.response.status_code", httpCode))

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Error, http.StatusText(httpCode))
	}
}
