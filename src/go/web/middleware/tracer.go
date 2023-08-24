package middleware

import (
	"context"
	"net/http"
	"time"

	"phenix/util/plog"

	"github.com/gofrs/uuid"
)

var (
	RequestIDKey struct{}
	SpansKey     struct{}
)

type Span struct {
	Name     string
	Duration time.Duration
}

func Trace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			id  = r.Header.Get("X-Request-Id")
			ctx = r.Context()
		)

		if id == "" {
			id = uuid.Must(uuid.NewV4()).String()
		}

		spans := make([]*Span, 0)

		ctx = context.WithValue(ctx, RequestIDKey, id)
		ctx = context.WithValue(ctx, SpansKey, spans)

		w.Header().Set("X-Request-Id", id)

		begin := time.Now()
		next.ServeHTTP(w, r.WithContext(ctx))
		end := time.Now()

		user, ok := r.Context().Value("user").(string)
		if !ok {
			user = "unknown"
		}

		plog.Debug("HTTP Trace", "id", id, "user", user, "method", r.Method, "path", r.URL.Path, "duration", end.Sub(begin))

		for _, span := range spans {
			plog.Debug("HTTP Trace", "id", id, "span", span.Name, "duration", span.Duration)
		}
	})
}

func NewSpan(ctx context.Context) *Span {
	spans, ok := ctx.Value(SpansKey).([]*Span)
	if !ok {
		return nil
	}

	span := new(Span)
	_ = append(spans, span)

	return span
}
