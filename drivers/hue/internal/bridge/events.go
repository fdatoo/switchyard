package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// sseEnvelope is the outer JSON object emitted by the bridge in each
// "data:" frame. The inner Data slice carries the partial resource updates
// we care about.
type sseEnvelope struct {
	Type string  `json:"type"`
	Data []Event `json:"data"`
}

// Events opens the bridge's SSE stream and returns a channel of resource
// update events. The channel is closed when the stream disconnects, the
// HTTP body returns EOF, or ctx is cancelled. Callers should range over the
// channel until it closes; reconnect is the caller's responsibility.
func (c *Client) Events(ctx context.Context) (<-chan Event, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/eventstream/clip/v2", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("hue-application-key", c.apiKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req) //nolint:bodyclose // body is consumed and closed inside the goroutine below
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("hue: events: status %d: %s", resp.StatusCode, body)
	}

	out := make(chan Event, 16)
	go func() {
		defer close(out)
		defer resp.Body.Close() //nolint:errcheck // body read until EOF or ctx cancel
		scanner := bufio.NewScanner(resp.Body)
		// SSE frames can be larger than the default 64 KiB buffer.
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			payload := strings.TrimPrefix(line, "data: ")
			var envelopes []sseEnvelope
			if err := json.Unmarshal([]byte(payload), &envelopes); err != nil {
				// Malformed frame — skip and keep reading.
				continue
			}
			for _, env := range envelopes {
				if env.Type != "update" {
					continue
				}
				for _, ev := range env.Data {
					switch ev.Type {
					case "light", "zigbee_connectivity":
					default:
						continue
					}
					select {
					case out <- ev:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()
	return out, nil
}
