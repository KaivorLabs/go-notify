// SPDX-License-Identifier: Apache-2.0

package notify

import (
	"context"
	"fmt"
	"net/url"
)

// discordHookBase is the webhook URL prefix. A package var (never reassigned
// at runtime) so tests can redirect it.
var discordHookBase = "https://discord.com/api/webhooks/"

// Discord sends a message to a webhook. The webhook URL is
// .../webhooks/<WebhookID>/<Token>.
type Discord struct {
	WebhookID string // webhook id (the numeric segment of the webhook URL)
	Token     string // webhook token
	Text      string // message content
}

func (m Discord) send(ctx context.Context, n *Notifier) error {
	switch {
	case m.WebhookID == "":
		return missing("discord webhook id")
	case m.Token == "":
		return missing("discord webhook token")
	case m.Text == "":
		return missing("discord text")
	}
	// Both segments are escaped so neither can inject extra path/query and
	// misroute the request.
	urlStr := discordHookBase + url.PathEscape(m.WebhookID) + "/" + url.PathEscape(m.Token)
	code, body, err := n.postJSON(ctx, urlStr, map[string]string{"content": m.Text})
	if err != nil {
		return err
	}
	if code < 200 || code >= 300 {
		return fmt.Errorf("notify: discord send failed (http %d): %s", code, string(body))
	}
	return nil
}
