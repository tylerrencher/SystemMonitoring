package alerts

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/tylerrencher/systemmonitoring/internal/config"
)

type namedAlert struct {
	Key  string
	Name string
}

// sendRollup sends one email listing all fired alerts. Fails fast after 15 seconds.
func sendRollup(cfg *config.Config, toEmail string, alerts []namedAlert) error {
	done := make(chan error, 1)
	go func() { done <- doSend(cfg, toEmail, alerts) }()
	select {
	case err := <-done:
		return err
	case <-time.After(15 * time.Second):
		return fmt.Errorf("smtp timeout after 15s")
	}
}

func doSend(cfg *config.Config, toEmail string, alerts []namedAlert) error {
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	c, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer c.Close()

	if err := c.StartTLS(&tls.Config{ServerName: cfg.SMTPHost}); err != nil {
		return fmt.Errorf("starttls: %w", err)
	}

	auth := smtp.PlainAuth("", cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPHost)
	if err := c.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}

	if err := c.Mail(cfg.SMTPFrom); err != nil {
		return fmt.Errorf("smtp MAIL: %w", err)
	}
	if err := c.Rcpt(toEmail); err != nil {
		return fmt.Errorf("smtp RCPT: %w", err)
	}

	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}

	if _, err := fmt.Fprint(w, buildMessage(cfg.SMTPFrom, toEmail, alerts)); err != nil {
		return err
	}
	return w.Close()
}

func buildMessage(from, to string, alerts []namedAlert) string {
	var body strings.Builder
	body.WriteString("The following alerts are active on your home energy monitor:\r\n\r\n")
	for _, a := range alerts {
		fmt.Fprintf(&body, "  * %s\r\n", a.Name)
	}
	body.WriteString("\r\nLog in to the dashboard to view details and manage alerts.\r\n")

	subject := fmt.Sprintf("Home Monitor: %d Active Alerts", len(alerts))
	if len(alerts) == 1 {
		subject = fmt.Sprintf("Home Monitor: %s", alerts[0].Name)
	}

	return fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body.String(),
	)
}
