// SPDX-License-Identifier: Apache-2.0

package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// slackPostURL is the chat.postMessage endpoint. A package var (never
// reassigned at runtime) so tests can redirect it.
var slackPostURL = "https://slack.com/api/chat.postMessage"

// Slack sends a message via the chat.postMessage Web API method.
type Slack struct {
	Token   string // bot/user token
	Channel string // channel id
	Text    string // message text
}

type slackResp struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

func (m Slack) send(ctx context.Context, n *Notifier) error {
	switch {
	case m.Token == "":
		return missing("slack token")
	case m.Channel == "":
		return missing("slack channel")
	case m.Text == "":
		return missing("slack text")
	}
	form := url.Values{"token": {m.Token}, "channel": {m.Channel}, "text": {m.Text}}
	// Slack returns HTTP 200 even on logical failure, so the "ok" field in the
	// JSON body is authoritative.
	_, body, err := n.postForm(ctx, slackPostURL, form)
	if err != nil {
		return err
	}
	var r slackResp
	_ = json.Unmarshal(body, &r)
	if !r.OK {
		return fmt.Errorf("notify: slack send failed: %s", r.Error)
	}
	return nil
}
