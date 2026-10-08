// Package mail sends the app's transactional emails (address verification,
// password reset) through Cloudflare Email Service's REST API.
package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type Sender struct {
	accountID, token string
	from, fromName   string
	log              *slog.Logger
	http             *http.Client
}

// New returns a sender; without an account id and token it only logs each
// email (links included), so signup stays testable on a dev box.
func New(accountID, token, from, fromName string, log *slog.Logger) *Sender {
	return &Sender{accountID: accountID, token: token, from: from, fromName: fromName, log: log,
		http: &http.Client{Timeout: 15 * time.Second}}
}

func (s *Sender) Enabled() bool { return s.accountID != "" && s.token != "" }

type message struct {
	To      string `json:"to"`
	From    from   `json:"from"`
	Subject string `json:"subject"`
	Text    string `json:"text"`
	HTML    string `json:"html,omitempty"`
}

type from struct {
	Address string `json:"address"`
	Name    string `json:"name,omitempty"`
}

func (s *Sender) Send(ctx context.Context, to, subject, text, html string) error {
	if !s.Enabled() {
		s.log.Warn("email non envoyé (Cloudflare non configuré)", "to", to, "subject", subject, "text", text)
		return nil
	}
	body, err := json.Marshal(message{To: to, From: from{s.from, s.fromName}, Subject: subject, Text: text, HTML: html})
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/email/sending/send", s.accountID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")
	res, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare email : %w", err)
	}
	defer res.Body.Close()
	var out struct {
		Success bool `json:"success"`
		Errors  []struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if err := json.Unmarshal(raw, &out); err != nil || !out.Success {
		return fmt.Errorf("cloudflare email : HTTP %d : %s", res.StatusCode, raw)
	}
	return nil
}
