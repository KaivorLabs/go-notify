# go-notify

[![CI](https://github.com/KaivorLabs/go-notify/actions/workflows/ci.yml/badge.svg)](https://github.com/KaivorLabs/go-notify/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/KaivorLabs/go-notify.svg)](https://pkg.go.dev/github.com/KaivorLabs/go-notify)
[![Go Report Card](https://goreportcard.com/badge/github.com/KaivorLabs/go-notify)](https://goreportcard.com/report/github.com/KaivorLabs/go-notify)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

Send a short notification to one of several chat / alerting platforms using
**only the Go standard library** — no third-party dependencies.

Supported platforms: **Telegram, Slack, Discord, Lark (Feishu), DingTalk, Pushover, PagerDuty, Email (SMTP)**.

## Install

```sh
go get github.com/KaivorLabs/go-notify
```

## Usage

Each platform is its own message type, so every field is named for what it
means — pass one to `Send`:

```go
package main

import (
	"context"
	"log"

	notify "github.com/KaivorLabs/go-notify"
)

func main() {
	err := notify.Send(context.Background(), notify.Slack{
		Token:   "xoxb-...",  // bot/user token
		Channel: "C0123456",  // channel id
		Text:    "deploy finished",
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

Every send is driven by a `context.Context`, so the caller controls
cancellation and deadlines. The package-level `Send` uses `DefaultClient`
(a `*http.Client` with a 15s timeout). For a custom client or a deterministic
clock, construct a `Notifier`:

```go
n := &notify.Notifier{HTTPClient: myClient}
err := n.Send(ctx, notify.Telegram{Token: "...", ChatID: "@ops", Text: "hi"})
```

## Message types

```go
notify.Telegram { Token, ChatID, Text }                       // ChatID: numeric id or "@channelname"
notify.Slack    { Token, Channel, Text }
notify.Discord  { WebhookID, Token, Text }                    // URL: .../webhooks/<WebhookID>/<Token>
notify.Lark     { WebhookURL, Text }
notify.DingTalk { WebhookURL, Secret, Text }                  // WebhookURL typically includes ?access_token=...
notify.Pushover { Token, UserKey, Text }
notify.PagerDuty{ RoutingKey, Source, Severity, Summary }     // Severity empty → "critical"
notify.Email    { Host, Username, Password, To, Subject, Body }// Host is "host:port"; Username is the From address
```

> **Email note:** the SMTP connection is dialled with the `context.Context` and
> the context deadline is applied to the exchange, so cancellation and timeouts
> are honoured — set a deadline on `ctx` to bound a slow server. STARTTLS is used
> when the server offers it. To/From/Subject containing CR or LF are rejected
> (header-injection guard).

## Errors

A missing required field returns an error wrapping the `ErrMissingField`
sentinel — test for it with `errors.Is`. A platform-level rejection (e.g. Slack
`channel_not_found`, a non-2xx webhook response) is returned as a descriptive
error that includes the upstream reason.

## Design

- **Zero dependencies** — standard library only.
- **Context-first** — cancellation and deadlines are honoured per request.
- **Bounded** — response bodies are read with a 1 MiB cap.
- The fixed base URLs are package variables so tests can redirect them at an
  `httptest` server; they are never reassigned at runtime.

## License

[Apache License 2.0](LICENSE) © KaivorLabs. See [NOTICE](NOTICE).
