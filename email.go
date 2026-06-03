// SPDX-License-Identifier: Apache-2.0

package notify

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
)

// smtpSend performs the actual SMTP send. It is a package var so tests can
// stub it — there is no in-process SMTP server seam otherwise.
var smtpSend = sendSMTPContext

// Email sends a plain-text email over SMTP.
//
// The send honours ctx: the connection is dialled with the context and the
// context deadline is applied to the SMTP exchange (unlike smtp.SendMail).
// STARTTLS is used when the server offers it.
type Email struct {
	Host     string // SMTP server "host:port"
	Username string // SMTP auth user; also used as the From address
	Password string // SMTP auth password
	To       string // recipient address
	Subject  string // subject line
	Body     string // message body
}

func (m Email) send(ctx context.Context, n *Notifier) error {
	switch {
	case m.Host == "":
		return missing("email smtp host:port")
	case m.Username == "":
		return missing("email from/username")
	case m.To == "":
		return missing("email recipient")
	case m.Body == "":
		return missing("email body")
	}
	// Reject CR/LF in any value that lands in a header line, to prevent header
	// injection (e.g. a smuggled "\r\nBcc:" via Subject or recipient).
	for _, v := range []string{m.To, m.Username, m.Subject} {
		if strings.ContainsAny(v, "\r\n") {
			return errors.New("notify: email header field contains CR or LF")
		}
	}
	// PlainAuth wants the bare hostname; SplitHostPort handles IPv6 correctly
	// and falls back to the raw value when no port is present.
	host := m.Host
	if h, _, err := net.SplitHostPort(m.Host); err == nil {
		host = h
	}
	auth := smtp.PlainAuth("", m.Username, m.Password, host)
	msg := []byte("To: " + m.To + "\r\n" +
		"From: " + m.Username + "\r\n" +
		"Subject: " + m.Subject + "\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" + m.Body)
	if err := smtpSend(ctx, m.Host, auth, m.Username, []string{m.To}, msg); err != nil {
		return fmt.Errorf("notify: email send failed: %w", err)
	}
	return nil
}

// sendSMTPContext mirrors smtp.SendMail but dials with ctx and applies the
// context deadline to the connection, so the SMTP exchange honours
// cancellation and timeouts. It upgrades to TLS via STARTTLS when the server
// offers it, and authenticates only when AUTH is advertised.
func sendSMTPContext(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	if err := validateSMTPLine(from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := validateSMTPLine(rcpt); err != nil {
			return err
		}
	}
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer c.Close()
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: host}); err != nil {
			return err
		}
	}
	if auth != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(auth); err != nil {
				return err
			}
		}
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := c.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// validateSMTPLine rejects CR/LF in an SMTP command argument (envelope
// from/recipient), mirroring net/smtp's own line validation.
func validateSMTPLine(s string) error {
	if strings.ContainsAny(s, "\r\n") {
		return errors.New("notify: smtp address contains CR or LF")
	}
	return nil
}
