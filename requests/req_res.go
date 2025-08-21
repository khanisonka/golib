package requests

import (
	"bytes"
	"io"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type traceTransport struct {
	base     http.RoundTripper
	maxBytes int
}

func (t *traceTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	span := trace.SpanFromContext(req.Context())

	if req.Body != nil && req.Body != http.NoBody {
		b, _ := io.ReadAll(io.LimitReader(req.Body, int64(t.maxBytes)))
		req.Body = io.NopCloser(bytes.NewReader(b))
		span.SetAttributes(attribute.String("http.request.body", truncate(string(b), t.maxBytes)))
	}

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	if resp.Body != nil && resp.Body != http.NoBody {
		buf := new(bytes.Buffer)
		tee := io.TeeReader(resp.Body, buf)

		b, _ := io.ReadAll(io.LimitReader(tee, int64(t.maxBytes)))

		resp.Body.Close()
		resp.Body = io.NopCloser(buf)

		span.SetAttributes(
			attribute.Int("http.response.status_code", resp.StatusCode),
			attribute.String("http.response.body", truncate(string(b), t.maxBytes)),
		)
	}

	return resp, nil
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max] + "...(truncated)"
	}
	return s
}
