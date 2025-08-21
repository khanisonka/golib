package requests

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var DefaultTLSConfig = tls.Config{InsecureSkipVerify: true}

func Request(ctx context.Context, method string, url string, headers map[string]string, body io.Reader, timeout int) (Response, error) {
	return RequestWithTLSConfig(ctx, method, url, headers, body, timeout, nil)
}

func RequestWithTLSConfig(ctx context.Context, method string, url string, headers map[string]string, body io.Reader, timeout int, tlsCfg *tls.Config) (Response, error) {
	if timeout == 0 {
		timeout = Timeout
	}
	r := Response{}

	if ctx == nil {
		ctx = context.Background()
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return r, err
	}

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	if tlsCfg == nil {
		tlsCfg = &DefaultTLSConfig
	}

	transport := &http.Transport{
		TLSClientConfig: tlsCfg,
	}

	client := &http.Client{
		Transport: &captureTransport{
			base:     otelhttp.NewTransport(transport),
			maxBytes: 2048,
		},
		Timeout: time.Duration(timeout) * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return r, err
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)

	r.Code = resp.StatusCode
	r.Status = resp.Status
	r.Body = buf.Bytes()
	r.Header = resp.Header

	return r, nil
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
