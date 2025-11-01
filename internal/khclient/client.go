package khclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"

	"kh/internal/config"
)

type Client struct {
	Endpoint string
	Token    string
	Org      string
	HTTP     *http.Client
}

func New(cfg config.Config) *Client {
	c := &Client{
		Endpoint: cfg.Endpoint,
		Token:    cfg.Token,
		Org:      cfg.Org,
		HTTP:     &http.Client{Timeout: 30 * time.Second},
	}
	return c
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
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		req, err = http.NewRequestWithContext(ctx, method, u.String(), bytesReader(b))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		var err error
		req, err = http.NewRequestWithContext(ctx, method, u.String(), nil)
		if err != nil {
			return nil, err
		}
	}
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
