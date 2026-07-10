package diag

import (
	"context"
	"log/slog"
)

// Redaction in v1.1 is performed by redactAttr, installed as
// slog.HandlerOptions.ReplaceAttr on the base TextHandler. handlerWrapper keeps
// the structural wrapping seam for future handler changes without moving PII
// decisions into producers.

// Redacts the six D-21 PII fields only. device_id and URLs/content-ids are NOT
// redacted.
var redactKeys = map[string]struct{}{"token": {}, "cookie": {}, "etp_rt": {}, "client_id": {}, "private_key": {}, "authorization": {}}

func redactAttr(groups []string, a slog.Attr) slog.Attr {
	if _, ok := redactKeys[a.Key]; ok {
		return slog.String(a.Key, "[REDACTED]")
	}
	return a
}

type handlerWrapper struct {
	inner slog.Handler
}

func (h handlerWrapper) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h handlerWrapper) Handle(ctx context.Context, record slog.Record) error {
	return h.inner.Handle(ctx, record)
}

func (h handlerWrapper) WithAttrs(attrs []slog.Attr) slog.Handler {
	return handlerWrapper{inner: h.inner.WithAttrs(attrs)}
}

func (h handlerWrapper) WithGroup(name string) slog.Handler {
	return handlerWrapper{inner: h.inner.WithGroup(name)}
}
