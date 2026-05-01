package khclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"path"
	"time"

	"kh/internal/config"
	"kh/internal/logging"
)

const (
	defaultRetryCount = 2
	defaultRetryWait  = 200 * time.Millisecond
	defaultTimeout    = 30 * time.Second
)

type requestBody struct {
	data        []byte
	contentType string
}

func rawDataBody(data []byte, contentType string) requestBody {
	return requestBody{data: data, contentType: contentType}
}

type Client struct {
	Endpoint  string
	Token     string
	Org       string
	HTTP      *http.Client
	Retries   int
	RetryWait time.Duration
}

func New(cfg config.Config) *Client {
	httpClient := &http.Client{Timeout: defaultTimeout}
	if cfg.InsecureTLS {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // intentional, gated behind KH_INSECURE=1
		}
	}
	return &Client{
		Endpoint:  cfg.Endpoint,
		Token:     cfg.Token,
		Org:       cfg.Org,
		HTTP:      httpClient,
		Retries:   defaultRetryCount,
		RetryWait: defaultRetryWait,
	}
}

func (c *Client) do(ctx context.Context, method, p string, q url.Values, body any, headers map[string]string) (*http.Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	attempts := c.Retries + 1
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		req, err := c.newReq(ctx, method, p, q, body)
		if err != nil {
			return nil, err
		}
		for k, v := range headers {
			if v == "" {
				continue
			}
			req.Header.Set(k, v)
		}
		if logging.Enabled() {
			logging.Debugf("HTTP %s %s", method, req.URL.String())
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			if attempt == attempts-1 || !shouldRetryErr(err) {
				return nil, err
			}
			time.Sleep(c.retryDelay(attempt))
			continue
		}
		if logging.Enabled() {
			logging.Debugf("HTTP status %d for %s %s", resp.StatusCode, method, req.URL.String())
		}
		if shouldRetryStatus(resp.StatusCode) {
			lastErr = fmt.Errorf("temporary status: %s", resp.Status)
			resp.Body.Close()
			if attempt == attempts-1 {
				return nil, lastErr
			}
			time.Sleep(c.retryDelay(attempt))
			continue
		}
		return resp, nil
	}
	if lastErr == nil {
		lastErr = errors.New("request failed after retries")
	}
	return nil, lastErr
}

func (c *Client) retryDelay(attempt int) time.Duration {
	wait := c.RetryWait
	if wait <= 0 {
		wait = defaultRetryWait
	}
	backoff := 1 << attempt
	base := time.Duration(backoff) * wait
	// Add ±20% jitter to spread retries under load and avoid thundering herd.
	jitter := time.Duration(rand.Int63n(int64(base/5)*2+1) - int64(base/5)) //nolint:gosec // retry jitter; crypto randomness not needed
	return base + jitter
}

func shouldRetryErr(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}
	return errors.Is(err, context.DeadlineExceeded)
}

func shouldRetryStatus(code int) bool {
	if code == 429 || code == http.StatusRequestTimeout {
		return true
	}
	return code >= 500 && code != http.StatusNotImplemented
}

func (c *Client) newReq(ctx context.Context, method, p string, q url.Values, body any) (*http.Request, error) {
	if c.Endpoint == "" {
		return nil, fmt.Errorf("missing endpoint in config")
	}
	u, err := url.Parse(c.Endpoint)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, p)
	u.RawQuery = q.Encode()

	var req *http.Request
	switch v := body.(type) {
	case nil:
		var err error
		req, err = http.NewRequestWithContext(ctx, method, u.String(), nil)
		if err != nil {
			return nil, err
		}
	case requestBody:
		var err error
		req, err = http.NewRequestWithContext(ctx, method, u.String(), bytesReader(v.data))
		if err != nil {
			return nil, err
		}
		if v.contentType != "" {
			req.Header.Set("Content-Type", v.contentType)
		}
	default:
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		req, err = http.NewRequestWithContext(ctx, method, u.String(), bytesReader(b))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if c.Org != "" {
		req.Header.Set("X-Org", c.Org)
	}
	return req, nil
}

// tiny helper to avoid importing bytes everywhere
func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }
