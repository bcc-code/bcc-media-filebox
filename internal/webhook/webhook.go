// Package webhook sends upload notifications to a target's configured URL.
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// client is shared across sends; the timeout bounds a slow or unresponsive
// receiver so a stuck webhook never wedges the upload goroutine (or, for the
// admin re-trigger, the HTTP request handler).
var client = &http.Client{Timeout: 15 * time.Second}

// Payload is the data sent to a webhook. Sidecar is the JSON sidecar's base
// filename; Path is its location relative to the upload root.
type Payload struct {
	Sidecar string `json:"sidecar"`
	Path    string `json:"path"`
}

// Send POSTs the payload to rawURL as a JSON body
// ({"sidecar":"<name>","path":"<relpath>"}). A non-2xx response is treated as
// an error.
func Send(ctx context.Context, rawURL string, p Payload) error {
	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer resp.Body.Close()
	// Drain so the connection can be reused.
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
