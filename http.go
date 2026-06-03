// SPDX-License-Identifier: Apache-2.0

package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// post issues a POST and returns the status code and a size-capped body.
// The response body is always closed.
func (n *Notifier) post(ctx context.Context, urlStr, contentType string, body io.Reader) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, body)
	if err != nil {
		return 0, nil, fmt.Errorf("notify: build request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := n.client().Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("notify: send request: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxRespBody))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("notify: read response: %w", err)
	}
	return resp.StatusCode, b, nil
}

func (n *Notifier) postForm(ctx context.Context, urlStr string, form url.Values) (int, []byte, error) {
	return n.post(ctx, urlStr, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
}

func (n *Notifier) postJSON(ctx context.Context, urlStr string, payload any) (int, []byte, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, fmt.Errorf("notify: marshal payload: %w", err)
	}
	return n.post(ctx, urlStr, "application/json", bytes.NewReader(b))
}
