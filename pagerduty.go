// SPDX-License-Identifier: Apache-2.0

package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// pagerDutyURL is the Events API v2 enqueue endpoint. A package var (never
// reassigned at runtime) so tests can redirect it.
var pagerDutyURL = "https://events.pagerduty.com/v2/enqueue"

// PagerDuty triggers an event via the Events API v2.
type PagerDuty struct {
	RoutingKey string // integration "routing key"
	Source     string // the host/service that triggered the alert
	Severity   string // "critical" | "error" | "warning" | "info"; empty defaults to "critical"
	Summary    string // alert summary (the message body)
}

type pagerDutyResp struct {
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Errors  []string `json:"errors"`
}

func (m PagerDuty) send(ctx context.Context, n *Notifier) error {
	switch {
	case m.RoutingKey == "":
		return missing("pagerduty routing key")
	case m.Source == "":
		return missing("pagerduty source")
	case m.Summary == "":
		return missing("pagerduty summary")
	}
	severity := m.Severity
	if severity == "" {
		severity = "critical"
	}
	payload := map[string]any{
		"routing_key":  m.RoutingKey,
		"event_action": "trigger",
		"payload": map[string]string{
			"summary":  m.Summary,
			"source":   m.Source,
			"severity": severity,
		},
	}
	_, body, err := n.postJSON(ctx, pagerDutyURL, payload)
	if err != nil {
		return err
	}
	var r pagerDutyResp
	_ = json.Unmarshal(body, &r)
	if r.Status != "success" {
		detail := r.Message
		if len(r.Errors) > 0 {
			detail = strings.TrimPrefix(detail+": "+strings.Join(r.Errors, "; "), ": ")
		}
		if detail == "" {
			detail = r.Status
		}
		return fmt.Errorf("notify: pagerduty send failed: %s", detail)
	}
	return nil
}
