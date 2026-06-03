// SPDX-License-Identifier: Apache-2.0

package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// pushoverURL is the messages endpoint. A package var (never reassigned at
// runtime) so tests can redirect it.
var pushoverURL = "https://api.pushover.net/1/messages.json"

// Pushover sends a message via the Pushover messages API.
type Pushover struct {
	Token   string // application token
	UserKey string // user or group key
	Text    string // message text
}

type pushoverResp struct {
	Status int      `json:"status"`
	Errors []string `json:"errors"`
}

func (m Pushover) send(ctx context.Context, n *Notifier) error {
	switch {
	case m.Token == "":
		return missing("pushover application token")
	case m.UserKey == "":
		return missing("pushover user/group key")
	case m.Text == "":
		return missing("pushover text")
	}
	form := url.Values{"token": {m.Token}, "user": {m.UserKey}, "message": {m.Text}}
	code, body, err := n.postForm(ctx, pushoverURL, form)
	if err != nil {
		return err
	}
	var r pushoverResp
	_ = json.Unmarshal(body, &r)
	if r.Status != 1 {
		msg := "unknown error"
		if len(r.Errors) > 0 {
			msg = strings.Join(r.Errors, "; ")
		}
		return fmt.Errorf("notify: pushover send failed (http %d): %s", code, msg)
	}
	return nil
}
