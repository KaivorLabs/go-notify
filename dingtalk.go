// SPDX-License-Identifier: Apache-2.0

package notify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// DingTalk sends a text message to a custom-robot webhook with signed-request
// security enabled. The signature is HMAC-SHA256 over "<timestamp>\n<secret>",
// base64-encoded, added as timestamp/sign query parameters (preserving any
// existing query such as access_token).
type DingTalk struct {
	WebhookURL string // full webhook URL (typically includes ?access_token=...)
	Secret     string // signing secret
	Text       string // message text
}

type dingTalkResp struct {
	Errcode int    `json:"errcode"`
	Errmsg  string `json:"errmsg"`
}

func (m DingTalk) send(ctx context.Context, n *Notifier) error {
	switch {
	case m.WebhookURL == "":
		return missing("dingtalk webhook url")
	case m.Secret == "":
		return missing("dingtalk signing secret")
	case m.Text == "":
		return missing("dingtalk text")
	}
	u, err := url.Parse(m.WebhookURL)
	if err != nil {
		return fmt.Errorf("notify: dingtalk: invalid webhook url: %w", err)
	}
	timestamp := n.now().UnixMilli()
	mac := hmac.New(sha256.New, []byte(m.Secret))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10) + "\n" + m.Secret))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	// Merge into the existing query so a URL with or without "?access_token="
	// is handled correctly; Encode does the escaping.
	q := u.Query()
	q.Set("timestamp", strconv.FormatInt(timestamp, 10))
	q.Set("sign", sign)
	u.RawQuery = q.Encode()

	payload := map[string]any{"msgtype": "text", "text": map[string]string{"content": m.Text}}
	code, body, err := n.postJSON(ctx, u.String(), payload)
	if err != nil {
		return err
	}
	var r dingTalkResp
	_ = json.Unmarshal(body, &r)
	if r.Errcode != 0 {
		return fmt.Errorf("notify: dingtalk send failed (http %d): %s", code, r.Errmsg)
	}
	return nil
}
