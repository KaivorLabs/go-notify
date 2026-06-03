// SPDX-License-Identifier: Apache-2.0

package notify

import (
	"context"
	"encoding/json"
	"fmt"
)

// Lark sends a text message to a Feishu/Lark custom-bot webhook.
type Lark struct {
	WebhookURL string // full custom-bot webhook URL
	Text       string // message text
}

func (m Lark) send(ctx context.Context, n *Notifier) error {
	switch {
	case m.WebhookURL == "":
		return missing("lark webhook url")
	case m.Text == "":
		return missing("lark text")
	}
	// Lark's text message nests a JSON-encoded string under "content".
	inner, _ := json.Marshal(map[string]string{"text": m.Text})
	code, body, err := n.postJSON(ctx, m.WebhookURL, map[string]string{
		"msg_type": "text",
		"content":  string(inner),
	})
	if err != nil {
		return err
	}
	if code < 200 || code >= 300 {
		return fmt.Errorf("notify: lark send failed (http %d): %s", code, string(body))
	}
	return nil
}
