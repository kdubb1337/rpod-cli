package api

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

	"github.com/kdubb1337/rpod-cli/internal/output"
)

const (
	DefaultBaseURL = "https://rest.runpod.io/v1"
	DefaultTimeout = 30 * time.Second
	userAgent      = "rpod-cli"
)

// Client is a thin RunPod REST API client. Construct with New.
type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

// New builds a Client. APIKey resolution is the caller's job; see internal/config.
func New(apiKey string) *Client {
	return &Client{
		BaseURL: DefaultBaseURL,
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: DefaultTimeout},
	}
}

// APIError carries a structured upstream error and an exit code for the CLI.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	Body       string
	Exit       int
}

func (e *APIError) Error() string { return e.Message }
func (e *APIError) ExitCode() int { return e.Exit }

// exitFor maps an HTTP status to one of the CLI's typed exit codes.
func exitFor(status int) int {
	switch {
	case status == 401, status == 403:
		return 4
	case status == 404:
		return 3
	case status == 409:
		return 6
	case status == 422:
		return 9
	case status == 429:
		return 7
	case status >= 500:
		return 5
	default:
		return 1
	}
}

// Do executes an authenticated request and decodes JSON into out (if non-nil).
// body, if provided, is JSON-encoded.
func (c *Client) Do(ctx context.Context, method, path string, body any, out any) error {
	if c.APIKey == "" {
		return output.ErrorfHint(4, "auth_missing",
			"set RUNPOD_API_KEY or run `rpod auth add <key>`",
			"no API key configured")
	}

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	u := c.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	output.Debug("→ %s %s", method, u)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) && ue.Timeout() {
			return output.Errorf(124, "timeout", "request timed out: %s %s", method, u)
		}
		return output.Errorf(8, "network", "transport error: %v", err)
	}
	defer resp.Body.Close()

	rawBody, _ := io.ReadAll(resp.Body)
	output.Debug("← %d %s (%d bytes)", resp.StatusCode, http.StatusText(resp.StatusCode), len(rawBody))

	if resp.StatusCode >= 400 {
		return parseError(resp.StatusCode, rawBody)
	}
	if out == nil || len(rawBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(rawBody, out); err != nil {
		return output.Errorf(5, "decode_failed",
			"could not decode response body: %v (first 200 bytes: %q)",
			err, truncate(string(rawBody), 200))
	}
	return nil
}

func parseError(status int, body []byte) error {
	exit := exitFor(status)
	// RunPod errors typically come as {"error": "..."} or {"errors": [...]} or
	// {"message": "..."}; fall back to raw body when shape is unknown.
	var generic struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	msg := strings.TrimSpace(string(body))
	if err := json.Unmarshal(body, &generic); err == nil {
		switch {
		case generic.Error != "":
			msg = generic.Error
		case generic.Message != "":
			msg = generic.Message
		}
	}
	if msg == "" {
		msg = http.StatusText(status)
	}
	apiErr := &APIError{
		StatusCode: status,
		Code:       fmt.Sprintf("http_%d", status),
		Message:    msg,
		Body:       string(body),
		Exit:       exit,
	}
	return output.Errorf(exit, apiErr.Code, "%s (HTTP %d)", apiErr.Message, status)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
