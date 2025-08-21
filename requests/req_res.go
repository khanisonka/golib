package requests

import (
	"bytes"
	"io"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type captureTransport struct {
	base     http.RoundTripper
	maxBytes int
}

func (t *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	span := trace.SpanFromContext(req.Context())

	// ---- capture request body ----
	if req.Body != nil && req.Body != http.NoBody {
		b, _ := io.ReadAll(io.LimitReader(req.Body, int64(t.maxBytes)))
		req.Body = io.NopCloser(bytes.NewReader(b)) // reset body ให้ client ใช้ต่อได้
		span.SetAttributes(attribute.String("http.request.body", truncate(string(b), t.maxBytes)))
	}

	// ---- ส่ง request ต่อไปจริง ----
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	// ---- capture response body ----
	if resp.Body != nil && resp.Body != http.NoBody {
		buf := new(bytes.Buffer)
		tee := io.TeeReader(resp.Body, buf)

		b, _ := io.ReadAll(io.LimitReader(tee, int64(t.maxBytes)))

		resp.Body.Close()
		resp.Body = io.NopCloser(buf) // reset body ให้ caller อ่านต่อได้

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
