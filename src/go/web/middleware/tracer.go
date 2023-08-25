package middleware

import (
	"context"
	"net/http"
	"time"

	"phenix/util/plog"

	"github.com/gofrs/uuid"
)

type requestIDKey struct{}

type Span struct {
	Name     string
	Duration time.Duration
}

var spans = make(map[string][]*Span)

func Trace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			id  = r.Header.Get("X-Request-Id")
			ctx = r.Context()
		)

		if id == "" {
			id = uuid.Must(uuid.NewV4()).String()
		}

		ctx = context.WithValue(ctx, requestIDKey{}, id)

		w.Header().Set("X-Request-Id", id)

		begin := time.Now()
		next.ServeHTTP(w, r.WithContext(ctx))
		end := time.Now()

		user, ok := r.Context().Value("user").(string)
		if !ok {
			user = "unknown"
		}

		plog.Debug("HTTP Trace", "id", id, "user", user, "method", r.Method, "path", r.URL.Path, "duration", end.Sub(begin))

		for _, span := range spans[id] {
			plog.Debug("HTTP Trace", "id", id, "span", span.Name, "duration", span.Duration)
		}

		delete(spans, id)
	})
}

func NewSpan(ctx context.Context) *Span {
	trace, ok := ctx.Value(requestIDKey{}).(string)
	if !ok {
		return nil
	}

	span := new(Span)

	spans[trace] = append(spans[trace], span)

	return span
}
