// SPDX-License-Identifier: Apache-2.0

package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// telegramAPIBase is the Bot API host. A package var (never reassigned at
// runtime) so tests can point it at an httptest server.
var telegramAPIBase = "https://api.telegram.org"

// Telegram sends a message via the Bot API sendMessage method.
type Telegram struct {
	Token  string // bot token
	ChatID string // numeric chat id or "@channelname"
	Text   string // message text
}

type telegramResp struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

func (m Telegram) send(ctx context.Context, n *Notifier) error {
	switch {
	case m.Token == "":
		return missing("telegram bot token")
	case m.ChatID == "":
		return missing("telegram chat id")
	case m.Text == "":
		return missing("telegram text")
	}
	form := url.Values{"chat_id": {m.ChatID}, "text": {m.Text}}
	// Token goes into the URL path; escape it so a stray "/" or "?" cannot
	// silently redirect the request to a different path.
	endpoint := telegramAPIBase + "/bot" + url.PathEscape(m.Token) + "/sendMessage"
	code, body, err := n.postForm(ctx, endpoint, form)
	if err != nil {
		return err
	}
	var r telegramResp
	_ = json.Unmarshal(body, &r)
	if !r.OK {
		return fmt.Errorf("notify: telegram send failed (http %d): %s", code, r.Description)
	}
	return nil
}
