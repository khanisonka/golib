package requests

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
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

	var bodyBytes []byte
	if body != nil {
		bb, _ := io.ReadAll(body)
		bodyBytes = bb
	}

	tracer := otel.Tracer("external-api")
	ctxSpan, span := tracer.Start(ctx, "HTTP "+method+" "+url, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	req, err := http.NewRequestWithContext(ctxSpan, method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return Response{}, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	otel.GetTextMapPropagator().Inject(ctxSpan, propagation.HeaderCarrier(req.Header))

	const limit = 4096
	reqPreview := string(bodyBytes)
	if len(reqPreview) > limit {
		reqPreview = reqPreview[:limit] + "...truncated"
	}
	span.SetAttributes(
		attribute.String("method", method),
		attribute.String("url", url),
		attribute.String("request_body", reqPreview),
		attribute.Int("request_body_size", len(bodyBytes)),
	)

	if tlsCfg == nil {
		tlsCfg = &DefaultTLSConfig
	}
	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
		Timeout:   time.Duration(timeout) * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return Response{}, err
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	respBytes := buf.Bytes()

	respPreview := string(respBytes)
	if len(respPreview) > limit {
		respPreview = respPreview[:limit] + "...truncated"
	}

	span.SetAttributes(
		attribute.Int("status_code", resp.StatusCode),
		attribute.String("response_body", respPreview),
		attribute.Int("response_body_size", len(respBytes)),
	)
	if resp.StatusCode >= 400 {
		span.SetStatus(codes.Error, "http error")
	} else {
		span.SetStatus(codes.Ok, "OK")
	}

	return Response{
		Code:   resp.StatusCode,
		Status: resp.Status,
		Body:   respBytes,
		Header: resp.Header,
	}, nil
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
