package provider

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

// Client is the provider's own small REST client for the Cookie Munch Developer API. It speaks
// JSON, sends `Prefer: strict-unknown-fields` (a typo in a resource is an error, never silently
// dropped), pins the API version the provider was written against, and names itself.
type Client struct {
	BaseURL   string
	APIKey    string
	HTTP      *http.Client
	UserAgent string
}

// APIVersion is the dated API version this provider was written against.
const APIVersion = "2026-10-24"

// APIError is a non-2xx answer.
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("Cookie Munch API %d (%s): %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("Cookie Munch API %d: %s", e.Status, e.Message)
}

// IsNotFound reports whether err is a 404.
func IsNotFound(err error) bool {
	if e, ok := err.(*APIError); ok {
		return e.Status == http.StatusNotFound
	}
	return false
}

func newClient(baseURL, apiKey, version string) *Client {
	return &Client{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		APIKey:    apiKey,
		HTTP:      &http.Client{Timeout: 60 * time.Second},
		UserAgent: "terraform-provider-cookiemunch/" + version,
	}
}

// Do sends one request under /v1 and decodes a JSON answer into out (when out is non-nil).
func (c *Client) Do(ctx context.Context, method, path string, body any, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+"/v1"+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("CookieMunch-Version", APIVersion)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Prefer", "strict-unknown-fields")
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		var e struct {
			Error   string `json:"error"`
			Message string `json:"message"`
			Code    string `json:"code"`
		}
		_ = json.Unmarshal(raw, &e)
		msg := e.Error
		if e.Message != "" {
			msg = e.Message
		}
		if msg == "" {
			msg = strings.TrimSpace(string(raw))
		}
		return &APIError{Status: res.StatusCode, Code: e.Code, Message: msg}
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func esc(s string) string { return url.PathEscape(s) }
