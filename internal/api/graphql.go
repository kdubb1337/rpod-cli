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

	"github.com/kdubb1337/runpod-cli/internal/output"
)

// GraphQLEndpoint is RunPod's GraphQL surface — used for resources the REST API
// doesn't expose (e.g. gpuTypes).
const GraphQLEndpoint = "https://api.runpod.io/graphql"

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

// GraphQL executes a GraphQL query and unmarshals the `data` field into out.
// The API key is passed as a query string (`?api_key=...`) per RunPod docs.
// Many read-only queries (gpuTypes) work without authentication.
func (c *Client) GraphQL(ctx context.Context, query string, vars map[string]any, out any) error {
	body, err := json.Marshal(graphQLRequest{Query: query, Variables: vars})
	if err != nil {
		return fmt.Errorf("marshal graphql request: %w", err)
	}

	endpoint := GraphQLEndpoint
	if c.APIKey != "" {
		endpoint += "?api_key=" + url.QueryEscape(c.APIKey)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build graphql request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	output.Debug("→ POST %s (graphql, %d bytes query)", GraphQLEndpoint, len(query))
	resp, err := c.HTTP.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) && ue.Timeout() {
			return output.Errorf(124, "timeout", "graphql request timed out")
		}
		return output.Errorf(8, "network", "graphql transport error: %v", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	output.Debug("← %d (%d bytes)", resp.StatusCode, len(raw))

	if resp.StatusCode >= 400 {
		return output.Errorf(exitFor(resp.StatusCode), fmt.Sprintf("http_%d", resp.StatusCode),
			"graphql HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}

	var envelope graphQLResponse
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return output.Errorf(5, "decode_failed", "graphql decode failed: %v", err)
	}
	if len(envelope.Errors) > 0 {
		msgs := make([]string, len(envelope.Errors))
		for i, e := range envelope.Errors {
			msgs[i] = e.Message
		}
		return output.Errorf(5, "graphql_error", "graphql error: %v", msgs)
	}
	if out != nil && len(envelope.Data) > 0 {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return output.Errorf(5, "decode_failed", "decode graphql data: %v", err)
		}
	}
	return nil
}
