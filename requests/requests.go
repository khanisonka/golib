package requests

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var DefaultTLSConfig = tls.Config{InsecureSkipVerify: true}

func Request(ctx context.Context, method string, url string, headers map[string]string, body io.Reader, timeout int) (Response, error) {
	return RequestWithTLSConfig(ctx, method, url, headers, body, timeout, nil)
}

func RequestWithTLSConfig(ctx context.Context, method, url string, headers map[string]string, body io.Reader, timeout int, tlsCfg *tls.Config) (Response, error) {
	if timeout == 0 {
		timeout = Timeout
	}
	if ctx == nil {
		ctx = context.Background()
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return Response{}, err
	}
	for k, v := range headers {
		req.Header.Add(k, v)
	}

	if tlsCfg == nil {
		tlsCfg = &DefaultTLSConfig
	}
	transport := &http.Transport{TLSClientConfig: tlsCfg}
	otelWrapped := otelhttp.NewTransport(transport)

	client := &http.Client{
		Transport: otelWrapped,
		Timeout:   time.Duration(timeout) * time.Second,
	}

	const limit = 4096
	if span := trace.SpanFromContext(req.Context()); span != nil {
		if req.GetBody != nil {
			if rc, err := req.GetBody(); err == nil && rc != nil {
				b, _ := io.ReadAll(io.LimitReader(rc, int64(limit)))
				_ = rc.Close()
				span.SetAttributes(
					attribute.String("http.request.body", preview(string(b), limit)),
					attribute.Int("http.request.body.size", len(b)),
				)
			}
		}
		span.SetAttributes(
			attribute.String("http.method", method),
			attribute.String("http.url", url),
		)
	}

	resp, err := client.Do(req)
	if err != nil {
		if span := trace.SpanFromContext(req.Context()); span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return Response{}, err
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)

	r := Response{
		Code:   resp.StatusCode,
		Status: resp.Status,
		Body:   buf.Bytes(),
		Header: resp.Header,
	}

	if span := trace.SpanFromContext(req.Context()); span != nil {
		span.SetAttributes(
			attribute.Int("http.status_code", resp.StatusCode),
			attribute.String("http.response.body", preview(string(r.Body), limit)),
			attribute.Int("http.response.body.size", len(r.Body)),
		)
		if resp.StatusCode >= 400 {
			span.SetStatus(codes.Error, fmt.Sprintf("status=%d", resp.StatusCode))
		} else {
			span.SetStatus(codes.Ok, "OK")
		}
	}
	return r, nil
}

func preview(s string, limit int) string {
	if len(s) > limit {
		return s[:limit] + "...truncated"
	}
	return s
}

func Get(ctx context.Context, url string, headers map[string]string, body io.Reader, timeout int) (Response, error) {
	return Request(ctx, http.MethodGet, url, headers, body, timeout)
}

func Post(ctx context.Context, url string, headers map[string]string, body io.Reader, timeout int) (Response, error) {
	return Request(ctx, http.MethodPost, url, headers, body, timeout)
}

func Put(ctx context.Context, url string, headers map[string]string, body io.Reader, timeout int) (Response, error) {
	return Request(ctx, http.MethodPut, url, headers, body, timeout)
}

func Delete(ctx context.Context, url string, headers map[string]string, body io.Reader, timeout int) (Response, error) {
	return Request(ctx, http.MethodDelete, url, headers, body, timeout)
}
