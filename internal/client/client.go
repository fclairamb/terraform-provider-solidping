// Package client is a thin HTTP client over the SolidPing v1 REST API.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultTimeout = 30 * time.Second
	maxBodyBytes   = 8 << 20
)

// Client talks to one organization of one SolidPing server.
type Client struct {
	endpoint  string
	org       string
	token     string
	userAgent string
	http      *http.Client
}

// New builds a client. The endpoint is the server root (no `/api/v1` suffix).
func New(endpoint, org, token, userAgent string) *Client {
	return &Client{
		endpoint:  strings.TrimRight(endpoint, "/"),
		org:       org,
		token:     token,
		userAgent: userAgent,
		http:      &http.Client{Timeout: defaultTimeout},
	}
}

// Org returns the organization slug the client is bound to.
func (c *Client) Org() string { return c.org }

// FieldError is one entry of the `fields` list of a validation error.
type FieldError struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

// APIError is the `{title, code, detail}` error shape of the API.
type APIError struct {
	Status int          `json:"-"`
	Title  string       `json:"title"`
	Code   string       `json:"code"`
	Detail string       `json:"detail"`
	Fields []FieldError `json:"fields"`
}

func (e *APIError) Error() string {
	var sb strings.Builder

	sb.WriteString(e.Title)

	if e.Detail != "" && !strings.EqualFold(e.Detail, e.Title) {
		sb.WriteString(": " + e.Detail)
	}

	for _, f := range e.Fields {
		fmt.Fprintf(&sb, "\n  - %s: %s", f.Name, f.Message)
	}

	if e.Code != "" {
		fmt.Fprintf(&sb, " (%s, HTTP %d)", e.Code, e.Status)
	} else {
		fmt.Fprintf(&sb, " (HTTP %d)", e.Status)
	}

	return sb.String()
}

// IsNotFound tells whether err is a 404 from the API.
func IsNotFound(err error) bool {
	var apiErr *APIError

	return errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound
}

// orgPath builds `/api/v1/orgs/{org}/...` from path segments, escaping each.
func (c *Client) orgPath(segments ...string) string {
	parts := make([]string, 0, len(segments))
	for _, s := range segments {
		parts = append(parts, url.PathEscape(s))
	}

	return "/api/v1/orgs/" + url.PathEscape(c.org) + "/" + strings.Join(parts, "/")
}

// do sends a request and decodes the JSON response into out (when non-nil).
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader

	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request: %w", err)
		}

		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, reader)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}

	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return fmt.Errorf("%s %s: reading response: %w", method, path, err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return decodeError(resp.StatusCode, raw)
	}

	if out == nil || len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()

	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("%s %s: decoding response: %w", method, path, err)
	}

	return nil
}

func decodeError(status int, raw []byte) error {
	apiErr := &APIError{Status: status}

	if err := json.Unmarshal(raw, apiErr); err != nil || apiErr.Title == "" {
		apiErr.Title = http.StatusText(status)

		if text := strings.TrimSpace(string(raw)); text != "" && len(text) < 300 {
			apiErr.Detail = text
		}
	}

	return apiErr
}

type listEnvelope[T any] struct {
	Data []T `json:"data"`
}

// Version is the answer of `GET /api/mgmt/version`.
type Version struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

// GetVersion returns the version of the server.
func (c *Client) GetVersion(ctx context.Context) (*Version, error) {
	var out Version

	if err := c.do(ctx, http.MethodGet, "/api/mgmt/version", nil, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// Login exchanges credentials for a session access token (used by the
// acceptance tests to mint a personal access token).
func (c *Client) Login(ctx context.Context, email, password string) (string, error) {
	var out struct {
		AccessToken string `json:"accessToken"`
	}

	body := map[string]string{"org": c.org, "email": email, "password": password}

	if err := c.do(ctx, http.MethodPost, "/api/v1/auth/login", body, &out); err != nil {
		return "", err
	}

	return out.AccessToken, nil
}

// CreateToken mints a personal access token for the organization.
func (c *Client) CreateToken(ctx context.Context, name string) (string, error) {
	var out struct {
		Token string `json:"token"`
	}

	if err := c.do(ctx, http.MethodPost, c.orgPath("tokens"), map[string]string{"name": name}, &out); err != nil {
		return "", err
	}

	return out.Token, nil
}
