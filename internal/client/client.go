// Package client is the only package that talks HTTP to a Silo server.
//
// It owns authentication, ETag capture, cursor pagination, retry policy, and
// error mapping. Domain models and endpoint calls live in one file per API
// domain (access_groups.go, ...). See AGENTS.md for the contract rules this
// package implements: bearer auth, opaque string IDs, If-Match guarded
// writes, cursor pagination, and never retrying writes.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBasePath is the API prefix every Silo endpoint lives under.
const DefaultBasePath = "/api/v2"

const (
	// maxReadAttempts bounds retries on 429 and transport errors for reads
	// only. Writes are never retried: a dropped connection may have already
	// succeeded, and retrying could duplicate the resource.
	maxReadAttempts = 3
	// maxPageLimit is the API's documented upper bound for page size.
	maxPageLimit = 200
)

// Config carries the values the provider collects for the client.
type Config struct {
	// BaseURL is the server root, e.g. "https://silo.example.org".
	// It must not include the /api/v2 path.
	BaseURL string
	// APIKey is sent as a bearer token. Keys inherit their owner's
	// permissions; admin resources need an admin-owned key.
	APIKey string
}

// Validate normalizes the base URL and checks it is usable.
func (c Config) Validate() (Config, error) {
	raw := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if raw == "" {
		return c, fmt.Errorf("base_url is empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return c, fmt.Errorf("base_url %q is not a valid URL: %w", c.BaseURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return c, fmt.Errorf("base_url %q must use http or https", c.BaseURL)
	}
	if strings.HasSuffix(u.Path, DefaultBasePath) || strings.Contains(u.Path, DefaultBasePath+"/") {
		return c, fmt.Errorf("base_url %q must be the server root without the %s path", c.BaseURL, DefaultBasePath)
	}
	if strings.TrimSpace(c.APIKey) == "" {
		return c, fmt.Errorf("api_key is empty")
	}
	c.BaseURL = raw
	return c, nil
}

// Client is safe for concurrent use by the provider's goroutines.
type Client struct {
	cfg        Config
	httpClient *http.Client
}

// New builds a Client from validated Config.
func New(cfg Config) (*Client, error) {
	cfg, err := cfg.Validate()
	if err != nil {
		return nil, err
	}
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

// Response carries the parts of an HTTP response the provider keeps in
// state, chiefly the ETag used for If-Match guarded writes.
type Response struct {
	StatusCode int
	ETag       string
	RequestID  string
	Body       []byte
}

// request performs one HTTP round trip against the API. It applies bearer
// auth and optional headers. in, when non-nil, is marshalled as the JSON
// body. Non-2xx responses are returned as *APIError.
func (c *Client) request(ctx context.Context, method, path string, query url.Values, in any, headers map[string]string) (*Response, error) {
	endpoint := c.cfg.BaseURL + DefaultBasePath + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var body io.Reader
	if in != nil {
		buf, err := json.Marshal(in)
		if err != nil {
			return nil, fmt.Errorf("marshalling request body for %s %s: %w", method, path, err)
		}
		body = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("building request for %s %s: %w", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling %s %s: %w", method, path, err)
	}
	defer resp.Body.Close() //nolint:errcheck // read errors surfaced below

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response from %s %s: %w", method, path, err)
	}

	out := &Response{
		StatusCode: resp.StatusCode,
		ETag:       resp.Header.Get("ETag"),
		RequestID:  resp.Header.Get("X-Request-Id"),
		Body:       raw,
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return out, newAPIError(method, path, out)
	}
	return out, nil
}

// do is request plus retry policy. Reads (GET without an If-Match guard)
// retry on 429 and transport errors with linear backoff; writes never
// retry. out, when non-nil, is populated from a JSON response body.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, in, out any, headers map[string]string) (*Response, error) {
	readonly := method == http.MethodGet || method == http.MethodHead

	var resp *Response
	var err error
	for attempt := 0; attempt < maxReadAttempts; attempt++ {
		if attempt > 0 {
			// Back off and respect cancellation while waiting to retry.
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
			}
		}
		resp, err = c.request(ctx, method, path, query, in, headers)
		if err == nil {
			break
		}
		if apiErr, ok := err.(*APIError); ok {
			// 429 on a read: retry with the backoff above. Other status
			// codes are contract errors, not transient ones.
			if !(readonly && apiErr.Response.StatusCode == http.StatusTooManyRequests) {
				return resp, err
			}
			continue
		}
		// Transport error. Reads may retry; writes must not (the request
		// may have been applied before the connection dropped).
		if !readonly {
			return nil, err
		}
	}
	if err != nil {
		return resp, err
	}

	if out != nil && len(resp.Body) > 0 {
		if err := json.Unmarshal(resp.Body, out); err != nil {
			return resp, fmt.Errorf("parsing response from %s %s: %w", method, path, err)
		}
	}
	return resp, nil
}

// ifMatch returns the If-Match header value for a guarded write. An empty
// state etag means the resource was imported without one; "*" overwrites
// deliberately, which is the documented fallback.
func ifMatch(etag string) map[string]string {
	if strings.TrimSpace(etag) == "" {
		return map[string]string{"If-Match": "*"}
	}
	return map[string]string{"If-Match": etag}
}
