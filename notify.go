// SPDX-License-Identifier: Apache-2.0

// Package notify sends a short notification message to one of several chat
// or alerting platforms — Telegram, Slack, Discord, Lark (Feishu), DingTalk,
// Pushover, PagerDuty and Email (SMTP) — using only the Go standard library,
// with no third-party dependencies.
//
// Each platform is its own message type with its own fields, so there is no
// ambiguity about what a value means:
//
//	err := notify.Send(ctx, notify.Slack{
//	    Token:   "xoxb-...",
//	    Channel: "C0123456",
//	    Text:    "deploy finished",
//	})
//
// Every send is driven by a context.Context, so callers control cancellation
// and deadlines. The package-level [Send] uses [DefaultClient], an *http.Client
// with a 15s timeout; construct a [Notifier] for a custom client.
package notify

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Message is a platform-specific notification. The set of implementations is
// sealed to this package — construct one of the concrete types ([Telegram],
// [Slack], [Discord], [Lark], [DingTalk], [Pushover], [PagerDuty], [Email])
// and pass it to [Send] or [Notifier.Send].
type Message interface {
	// send delivers the message via n. It is unexported so the interface is
	// closed: callers use the concrete types, they cannot add new platforms.
	send(ctx context.Context, n *Notifier) error
}

// ErrMissingField is returned (wrapped, with the field name) when a required
// field on a message is empty. Test for it with errors.Is.
var ErrMissingField = errors.New("notify: missing required field")

// DefaultClient is used by [Send] and by a [Notifier] whose HTTPClient is nil.
// Its timeout bounds a single send; callers wanting different behaviour should
// set Notifier.HTTPClient.
var DefaultClient = &http.Client{Timeout: 15 * time.Second}

// maxRespBody caps how much of an upstream response is read.
const maxRespBody = 1 << 20

// Notifier sends messages with a configurable HTTP client. The zero value is
// usable and sends via DefaultClient.
type Notifier struct {
	// HTTPClient is used for all HTTP-based platforms. If nil, DefaultClient
	// is used. A client with no Timeout leaves each send bounded only by the
	// context deadline; set a Timeout for production use. (Email/SMTP does not
	// use this client; it is bounded by the context deadline directly.)
	HTTPClient *http.Client

	// Now returns the current time; only DingTalk's request signature depends
	// on it. If nil, time.Now is used. Set it to make signed requests
	// deterministic in tests.
	Now func() time.Time
}

func (n *Notifier) now() time.Time {
	if n.Now != nil {
		return n.Now()
	}
	return time.Now()
}

func (n *Notifier) client() *http.Client {
	if n.HTTPClient != nil {
		return n.HTTPClient
	}
	return DefaultClient
}

// Send delivers m using DefaultClient. It is a convenience wrapper around
// Notifier.Send.
func Send(ctx context.Context, m Message) error {
	return (&Notifier{}).Send(ctx, m)
}

// Send delivers m to its platform. The context bounds the request; on
// cancellation or deadline it returns ctx.Err() (wrapped).
func (n *Notifier) Send(ctx context.Context, m Message) error {
	if m == nil {
		return fmt.Errorf("%w: nil message", ErrMissingField)
	}
	return m.send(ctx, n)
}

// missing returns a wrapped ErrMissingField naming the empty field.
func missing(field string) error {
	return fmt.Errorf("%w: %s", ErrMissingField, field)
}
