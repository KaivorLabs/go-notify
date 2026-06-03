// SPDX-License-Identifier: Apache-2.0

package notify

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"net/url"
	"strings"
	"testing"
	"time"
)

// withTestServer points every fixed base URL at srv and restores the originals
// when the test finishes. Platforms whose endpoint comes from the message
// (Lark via WebhookURL, DingTalk via WebhookURL) are pointed at srv by the
// caller.
func withTestServer(t *testing.T, srv *httptest.Server) {
	t.Helper()
	origTg, origSl, origDc, origPo, origPd := telegramAPIBase, slackPostURL, discordHookBase, pushoverURL, pagerDutyURL
	telegramAPIBase = srv.URL
	slackPostURL = srv.URL + "/slack"
	discordHookBase = srv.URL + "/discord/"
	pushoverURL = srv.URL + "/pushover"
	pagerDutyURL = srv.URL + "/pagerduty"
	t.Cleanup(func() {
		telegramAPIBase, slackPostURL, discordHookBase, pushoverURL, pagerDutyURL = origTg, origSl, origDc, origPo, origPd
	})
}

// okHandler returns a per-platform success response keyed off the request path.
func okHandler(t *testing.T, got *http.Request) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		*got = *r
		switch {
		case strings.Contains(r.URL.Path, "/sendMessage"): // telegram
			io.WriteString(w, `{"ok":true}`)
		case strings.HasPrefix(r.URL.Path, "/slack"):
			io.WriteString(w, `{"ok":true}`)
		case strings.HasPrefix(r.URL.Path, "/discord/"):
			w.WriteHeader(http.StatusNoContent)
		case strings.HasPrefix(r.URL.Path, "/lark"):
			io.WriteString(w, `{}`)
		case strings.HasPrefix(r.URL.Path, "/dingtalk"):
			io.WriteString(w, `{"errcode":0}`)
		case strings.HasPrefix(r.URL.Path, "/pushover"):
			io.WriteString(w, `{"status":1}`)
		case strings.HasPrefix(r.URL.Path, "/pagerduty"):
			io.WriteString(w, `{"status":"success","dedup_key":"x"}`)
		default:
			t.Errorf("unexpected request path: %s", r.URL.Path)
			w.WriteHeader(http.StatusTeapot)
		}
	}
}

func TestSend_Success(t *testing.T) {
	var got http.Request
	srv := httptest.NewServer(okHandler(t, &got))
	defer srv.Close()
	withTestServer(t, srv)

	cases := []struct {
		name  string
		msg   Message
		check func(t *testing.T, r *http.Request)
	}{
		{
			name: "telegram",
			msg:  Telegram{Token: "bot123", ChatID: "@chan", Text: "hi"},
			check: func(t *testing.T, r *http.Request) {
				if r.URL.Path != "/botbot123/sendMessage" {
					t.Errorf("telegram path = %q", r.URL.Path)
				}
				if r.PostForm.Get("chat_id") != "@chan" || r.PostForm.Get("text") != "hi" {
					t.Errorf("telegram form = %v", r.PostForm)
				}
			},
		},
		{
			name: "slack",
			msg:  Slack{Token: "xoxb", Channel: "C1", Text: "hi"},
			check: func(t *testing.T, r *http.Request) {
				if r.PostForm.Get("token") != "xoxb" || r.PostForm.Get("channel") != "C1" {
					t.Errorf("slack form = %v", r.PostForm)
				}
			},
		},
		{
			name: "discord",
			msg:  Discord{WebhookID: "wid", Token: "tok", Text: "hi"},
			check: func(t *testing.T, r *http.Request) {
				if r.URL.Path != "/discord/wid/tok" {
					t.Errorf("discord path = %q", r.URL.Path)
				}
			},
		},
		{
			name: "lark",
			msg:  Lark{WebhookURL: srv.URL + "/lark", Text: "hi"},
			check: func(t *testing.T, r *http.Request) {
				if r.URL.Path != "/lark" {
					t.Errorf("lark path = %q", r.URL.Path)
				}
			},
		},
		{
			name: "dingtalk",
			msg:  DingTalk{Secret: "secret", WebhookURL: srv.URL + "/dingtalk?access_token=tok", Text: "hi"},
			check: func(t *testing.T, r *http.Request) {
				if r.URL.Path != "/dingtalk" {
					t.Errorf("dingtalk path = %q", r.URL.Path)
				}
				if r.URL.Query().Get("sign") == "" || r.URL.Query().Get("timestamp") == "" {
					t.Errorf("dingtalk missing sign/timestamp: %v", r.URL.RawQuery)
				}
			},
		},
		{
			name: "pushover",
			msg:  Pushover{Token: "apptok", UserKey: "userkey", Text: "hi"},
			check: func(t *testing.T, r *http.Request) {
				if r.PostForm.Get("user") != "userkey" {
					t.Errorf("pushover form = %v", r.PostForm)
				}
			},
		},
		{
			name: "pagerduty",
			msg:  PagerDuty{RoutingKey: "rk", Source: "host1", Summary: "boom"},
			check: func(t *testing.T, r *http.Request) {
				if r.URL.Path != "/pagerduty" {
					t.Errorf("pagerduty path = %q", r.URL.Path)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got = http.Request{}
			if err := Send(context.Background(), tc.msg); err != nil {
				t.Fatalf("Send(%s) = %v", tc.name, err)
			}
			if tc.check != nil {
				tc.check(t, &got)
			}
		})
	}
}

func TestSend_MissingField(t *testing.T) {
	cases := []struct {
		name string
		msg  Message
	}{
		{"nil message", nil},
		{"telegram no token", Telegram{ChatID: "c", Text: "hi"}},
		{"telegram no chat id", Telegram{Token: "t", Text: "hi"}},
		{"telegram no text", Telegram{Token: "t", ChatID: "c"}},
		{"slack no token", Slack{Channel: "c", Text: "hi"}},
		{"slack no channel", Slack{Token: "t", Text: "hi"}},
		{"discord no id", Discord{Token: "t", Text: "hi"}},
		{"discord no token", Discord{WebhookID: "w", Text: "hi"}},
		{"lark no url", Lark{Text: "hi"}},
		{"dingtalk no url", DingTalk{Secret: "s", Text: "hi"}},
		{"dingtalk no secret", DingTalk{WebhookURL: "https://x", Text: "hi"}},
		{"pushover no token", Pushover{UserKey: "u", Text: "hi"}},
		{"pushover no user", Pushover{Token: "t", Text: "hi"}},
		{"pagerduty no key", PagerDuty{Source: "s", Summary: "hi"}},
		{"pagerduty no source", PagerDuty{RoutingKey: "k", Summary: "hi"}},
		{"pagerduty no summary", PagerDuty{RoutingKey: "k", Source: "s"}},
		{"email no host", Email{Username: "u", To: "a@b", Body: "hi"}},
		{"email no username", Email{Host: "h:25", To: "a@b", Body: "hi"}},
		{"email no recipient", Email{Host: "h:25", Username: "u", Body: "hi"}},
		{"email no body", Email{Host: "h:25", Username: "u", To: "a@b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Send(context.Background(), tc.msg)
			if !errors.Is(err, ErrMissingField) {
				t.Fatalf("Send() error = %v, want errors.Is(..., ErrMissingField)", err)
			}
		})
	}
}

func TestSend_PlatformFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/slack"):
			io.WriteString(w, `{"ok":false,"error":"channel_not_found"}`)
		case strings.HasPrefix(r.URL.Path, "/discord/"):
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, "bad webhook")
		case strings.HasPrefix(r.URL.Path, "/pagerduty"):
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"status":"invalid event","message":"Event object is invalid","errors":["routing_key is invalid"]}`)
		default:
			io.WriteString(w, `{"ok":false}`)
		}
	}))
	defer srv.Close()
	withTestServer(t, srv)

	if err := Send(context.Background(), Slack{Token: "t", Channel: "c", Text: "hi"}); err == nil {
		t.Fatal("expected slack logical failure to return error")
	} else if !strings.Contains(err.Error(), "channel_not_found") {
		t.Errorf("slack error missing upstream reason: %v", err)
	}

	if err := Send(context.Background(), Discord{WebhookID: "w", Token: "t", Text: "hi"}); err == nil {
		t.Fatal("expected discord non-2xx to return error")
	}

	if err := Send(context.Background(), PagerDuty{RoutingKey: "rk", Source: "s", Summary: "hi"}); err == nil {
		t.Fatal("expected pagerduty failure to return error")
	} else if !strings.Contains(err.Error(), "routing_key is invalid") {
		t.Errorf("pagerduty error missing upstream reason: %v", err)
	}
}

func TestSend_PagerDutyPayload(t *testing.T) {
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		io.WriteString(w, `{"status":"success"}`)
	}))
	defer srv.Close()
	withTestServer(t, srv)

	// Severity omitted → must default to "critical".
	if err := Send(context.Background(), PagerDuty{RoutingKey: "rk", Source: "host1", Summary: "disk full"}); err != nil {
		t.Fatalf("Send = %v", err)
	}
	var got struct {
		RoutingKey  string `json:"routing_key"`
		EventAction string `json:"event_action"`
		Payload     struct {
			Summary  string `json:"summary"`
			Source   string `json:"source"`
			Severity string `json:"severity"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal body: %v (%s)", err, body)
	}
	if got.RoutingKey != "rk" || got.EventAction != "trigger" {
		t.Errorf("envelope = %+v", got)
	}
	if got.Payload.Summary != "disk full" || got.Payload.Source != "host1" || got.Payload.Severity != "critical" {
		t.Errorf("payload = %+v", got.Payload)
	}
}

func TestSend_DingTalkSignatureDeterministic(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		io.WriteString(w, `{"errcode":0}`)
	}))
	defer srv.Close()

	n := &Notifier{Now: func() time.Time { return time.Unix(1700000000, 0).UTC() }}
	err := n.Send(context.Background(), DingTalk{
		Secret: "SEC", WebhookURL: srv.URL + "/robot/send?access_token=tok", Text: "hi",
	})
	if err != nil {
		t.Fatalf("Send = %v", err)
	}
	q, _ := url.ParseQuery(query)
	if q.Get("timestamp") != "1700000000000" {
		t.Errorf("timestamp = %q, want 1700000000000", q.Get("timestamp"))
	}
	if q.Get("sign") == "" {
		t.Error("sign empty")
	}
}

// TestSend_DingTalkNoQueryWebhook verifies a webhook URL without an existing
// "?..." still gets timestamp/sign appended as a proper query.
func TestSend_DingTalkNoQueryWebhook(t *testing.T) {
	var gotQuery url.Values
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		io.WriteString(w, `{"errcode":0}`)
	}))
	defer srv.Close()

	n := &Notifier{Now: func() time.Time { return time.Unix(1700000000, 0).UTC() }}
	if err := n.Send(context.Background(), DingTalk{
		Secret: "SEC", WebhookURL: srv.URL + "/robot/send", Text: "hi",
	}); err != nil {
		t.Fatalf("Send = %v", err)
	}
	if gotPath != "/robot/send" {
		t.Errorf("path = %q, want /robot/send", gotPath)
	}
	if gotQuery.Get("timestamp") != "1700000000000" || gotQuery.Get("sign") == "" {
		t.Errorf("query = %v, want timestamp+sign", gotQuery)
	}
}

// TestSend_TelegramTokenEscaped verifies a "/" in the token is escaped so it
// cannot break out of the path segment, while a normal ":"-bearing token is
// preserved.
func TestSend_TelegramTokenEscaped(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath() // raw (still-encoded) path
		io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()
	withTestServer(t, srv)

	if err := Send(context.Background(), Telegram{Token: "123:ABC", ChatID: "@c", Text: "hi"}); err != nil {
		t.Fatalf("Send = %v", err)
	}
	if gotPath != "/bot123:ABC/sendMessage" {
		t.Errorf("path = %q, want /bot123:ABC/sendMessage", gotPath)
	}

	_ = Send(context.Background(), Telegram{Token: "a/b", ChatID: "@c", Text: "hi"})
	if !strings.Contains(gotPath, "%2F") {
		t.Errorf("escaped path = %q, want '/' escaped as %%2F", gotPath)
	}
}

func TestSend_ContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()
	withTestServer(t, srv)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Send(ctx, Slack{Token: "t", Channel: "c", Text: "hi"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Send with canceled ctx = %v, want context.Canceled", err)
	}
}

func TestSend_Email(t *testing.T) {
	var gotAddr, gotFrom string
	var gotTo []string
	var gotMsg []byte
	orig := smtpSend
	smtpSend = func(_ context.Context, addr string, _ smtp.Auth, from string, to []string, msg []byte) error {
		gotAddr, gotFrom, gotTo, gotMsg = addr, from, to, msg
		return nil
	}
	t.Cleanup(func() { smtpSend = orig })

	err := Send(context.Background(), Email{
		Host: "smtp.example.com:587", To: "ops@example.com",
		Username: "bot@example.com", Password: "pw", Subject: "Alert", Body: "disk full",
	})
	if err != nil {
		t.Fatalf("Send = %v", err)
	}
	if gotAddr != "smtp.example.com:587" {
		t.Errorf("addr = %q", gotAddr)
	}
	if gotFrom != "bot@example.com" {
		t.Errorf("from = %q", gotFrom)
	}
	if len(gotTo) != 1 || gotTo[0] != "ops@example.com" {
		t.Errorf("to = %v", gotTo)
	}
	s := string(gotMsg)
	for _, want := range []string{
		"To: ops@example.com", "From: bot@example.com", "Subject: Alert",
		"Content-Type: text/plain; charset=UTF-8", "\r\n\r\ndisk full",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("message missing %q in:\n%s", want, s)
		}
	}
}

func TestSend_EmailError(t *testing.T) {
	orig := smtpSend
	smtpSend = func(context.Context, string, smtp.Auth, string, []string, []byte) error {
		return errors.New("auth failed")
	}
	t.Cleanup(func() { smtpSend = orig })

	err := Send(context.Background(), Email{Host: "h:25", To: "a@b", Username: "u", Body: "hi"})
	if err == nil || !strings.Contains(err.Error(), "auth failed") {
		t.Fatalf("expected wrapped smtp error, got %v", err)
	}
}

// TestSend_EmailHeaderInjection guards against CR/LF smuggling extra headers
// (e.g. a hidden Bcc) through the recipient, From or Subject fields.
func TestSend_EmailHeaderInjection(t *testing.T) {
	orig := smtpSend
	called := false
	smtpSend = func(context.Context, string, smtp.Auth, string, []string, []byte) error {
		called = true
		return nil
	}
	t.Cleanup(func() { smtpSend = orig })

	bad := []Email{
		{Host: "h:25", To: "a@b\r\nBcc: evil@x", Username: "u", Body: "hi"},
		{Host: "h:25", To: "a@b", Username: "u\r\nBcc: evil@x", Body: "hi"},
		{Host: "h:25", To: "a@b", Username: "u", Subject: "s\r\nBcc: evil@x", Body: "hi"},
	}
	for i, m := range bad {
		if err := Send(context.Background(), m); err == nil {
			t.Errorf("case %d: expected CR/LF rejection, got nil", i)
		}
	}
	if called {
		t.Error("smtpSend must not be reached when a header field contains CR/LF")
	}
}

// TestSend_EmailIPv6Host verifies the SMTP auth hostname is parsed correctly
// for an IPv6 host:port (regression for the old strings.Split(host,":") bug).
func TestSend_EmailIPv6Host(t *testing.T) {
	var gotAddr string
	orig := smtpSend
	smtpSend = func(_ context.Context, addr string, _ smtp.Auth, _ string, _ []string, _ []byte) error {
		gotAddr = addr
		return nil
	}
	t.Cleanup(func() { smtpSend = orig })

	if err := Send(context.Background(), Email{
		Host: "[::1]:25", To: "a@b", Username: "u", Body: "hi",
	}); err != nil {
		t.Fatalf("Send = %v", err)
	}
	if gotAddr != "[::1]:25" {
		t.Errorf("addr = %q, want [::1]:25", gotAddr)
	}
}

// TestSendSMTPContext_Errors exercises the real SMTP send's validation and
// dial-failure branches without a live SMTP server.
func TestSendSMTPContext_Errors(t *testing.T) {
	if err := sendSMTPContext(context.Background(), "h:25", nil, "a\r\nb", []string{"x@y"}, nil); err == nil {
		t.Error("expected CR/LF rejection for from")
	}
	if err := sendSMTPContext(context.Background(), "h:25", nil, "a@b", []string{"x\r\ny"}, nil); err == nil {
		t.Error("expected CR/LF rejection for recipient")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sendSMTPContext(ctx, "127.0.0.1:0", nil, "a@b", []string{"x@y"}, nil); err == nil {
		t.Error("expected dial error on cancelled context")
	}
}
