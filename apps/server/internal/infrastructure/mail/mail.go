// Package mail renders Ricrac's emails and sends them over SMTP.
package mail

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strconv"
	"time"

	gomail "github.com/wneessen/go-mail"
)

// Config is where emails go out: an SMTP server and the sender.
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	// TLS from the start (smtps://, usually port 465) rather than STARTTLS (smtp://, usually 587).
	ImplicitTLS bool
	// Plain connections without TLS, for local test servers (smtp+insecure://).
	Insecure bool
	From     mail.Address
}

// ParseConfig reads SMTP_URL (smtps://user:password@host:465, smtp://user:password@host:587
// for STARTTLS, or smtp+insecure://host:1025 for a local test server) and EMAIL_FROM
// ("Ricrac <notifications@example.com>"). Both empty means email is off.
func ParseConfig(smtpURL, from string) (*Config, error) {
	if smtpURL == "" && from == "" {
		return nil, nil
	}
	if smtpURL == "" || from == "" {
		return nil, errors.New("set both SMTP_URL and EMAIL_FROM, or neither")
	}
	parsed, err := url.Parse(smtpURL)
	if err != nil || parsed.Hostname() == "" {
		return nil, errors.New("SMTP_URL must look like smtps://user:password@smtp.example.com:465")
	}
	config := &Config{Host: parsed.Hostname(), Username: parsed.User.Username()}
	config.Password, _ = parsed.User.Password()
	defaultPort := 587
	switch parsed.Scheme {
	case "smtps":
		config.ImplicitTLS, defaultPort = true, 465
	case "smtp":
	case "smtp+insecure":
		config.Insecure, defaultPort = true, 25
	default:
		return nil, errors.New("SMTP_URL starts with smtps://, smtp:// or smtp+insecure://")
	}
	config.Port = defaultPort
	if port := parsed.Port(); port != "" {
		if config.Port, err = strconv.Atoi(port); err != nil {
			return nil, errors.New("SMTP_URL has an invalid port")
		}
	}
	address, err := mail.ParseAddress(from)
	if err != nil {
		return nil, errors.New("EMAIL_FROM must be an address like \"Ricrac <notifications@example.com>\"")
	}
	config.From = *address
	return config, nil
}

type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// SMTP sends through the configured server.
type SMTP struct{ Config Config }

func (s SMTP) Send(ctx context.Context, msg Message) error {
	options := []gomail.Option{gomail.WithPort(s.Config.Port), gomail.WithTimeout(20 * time.Second)}
	auth := gomail.SMTPAuthAutoDiscover
	switch {
	case s.Config.ImplicitTLS:
		options = append(options, gomail.WithSSL())
	case s.Config.Insecure:
		// go-mail refuses PLAIN and LOGIN auth on a plaintext connection unless told to.
		options = append(options, gomail.WithTLSPolicy(gomail.NoTLS))
		auth = gomail.SMTPAuthPlainNoEnc
	default:
		options = append(options, gomail.WithTLSPolicy(gomail.TLSMandatory))
	}
	if s.Config.Username != "" {
		options = append(options, gomail.WithSMTPAuth(auth), gomail.WithUsername(s.Config.Username), gomail.WithPassword(s.Config.Password))
	}
	client, err := gomail.NewClient(s.Config.Host, options...)
	if err != nil {
		return fmt.Errorf("mail client: %w", err)
	}
	m, err := build(s.Config.From, msg)
	if err != nil {
		return err
	}
	return client.DialAndSendWithContext(ctx, m)
}

// build makes a plain multipart/alternative of the text and HTML.
func build(from mail.Address, msg Message) (*gomail.Msg, error) {
	// Spam filters score a User-Agent or X-Mailer header as mail sent straight from a client (DIRECT_TO_MX).
	m := gomail.NewMsg(gomail.WithNoDefaultUserAgent())
	if err := m.FromFormat(from.Name, from.Address); err != nil {
		return nil, fmt.Errorf("from address: %w", err)
	}
	if err := m.To(msg.To); err != nil {
		return nil, fmt.Errorf("to address: %w", err)
	}
	m.Subject(msg.Subject)
	m.SetBodyString(gomail.TypeTextPlain, msg.Text)
	m.AddAlternativeString(gomail.TypeTextHTML, msg.HTML)
	return m, nil
}
