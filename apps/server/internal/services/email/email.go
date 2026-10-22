// Package email_service queues emails in the transaction of the change that causes them and
// sends them from a job, so they only go out when the change happened. Notifications become
// digests: a person's unread ones are gathered into one email at most every digestInterval.
package email_service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"ricrac/config"
	"ricrac/internal/auth"
	"ricrac/internal/infrastructure/mail"
	repository "ricrac/internal/repositories"
	"ricrac/internal/repositories/repositories"
)

const (
	digestDelay    = 5 * time.Minute
	digestInterval = time.Hour
	digestMaxAge   = 24 * time.Hour
	digestMaxItems = 20
	excerptLength  = 400
	sendAttempts   = 5
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i EmailServiceInterface -p email_service -s EmailService -o email_service_iface.go
type EmailService struct {
	cfg    config.Config
	repo   repositories.RepositoriesInterface
	mailer mail.Sender
}

// NewEmailService sends over SMTP when the configuration has it; without a mailer nothing is sent.
func NewEmailService(cfg config.Config, repo repositories.RepositoriesInterface) *EmailService {
	var mailer mail.Sender
	if cfg.Mail != nil {
		mailer = mail.SMTP{Config: *cfg.Mail}
	}
	return &EmailService{cfg: cfg, repo: repo, mailer: mailer}
}

// SetMailer sends with sender from now on.
func (self *EmailService) SetMailer(sender mail.Sender) {
	self.mailer = sender
}

func (self *EmailService) queue(ctx context.Context, tx repository.TxInterface, to string, content mail.Content) error {
	encoded, err := json.Marshal(content)
	if err != nil {
		return err
	}
	return self.repo.Email().Queue(ctx, tx, to, encoded)
}

// QueueInvitation tells someone they were invited; they join by signing in as that email.
func (self *EmailService) QueueInvitation(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, to string, role auth.Role) error {
	inviter, workspace, err := self.repo.Email().Names(ctx, tx, actor.User, actor.Workspace)
	if err != nil {
		return err
	}
	article := "a"
	if role == auth.Admin {
		article = "an"
	}
	return self.queue(ctx, tx, to, mail.Content{
		Subject: fmt.Sprintf("%s invited you to %s on Ricrac", inviter, workspace),
		Heading: fmt.Sprintf("Join %s on Ricrac", workspace),
		Intro: []string{
			fmt.Sprintf("%s invited you to the %s workspace as %s %s.", inviter, workspace, article, role),
			fmt.Sprintf("Sign in as %s to accept.", to),
		},
		Button: mail.Button{Label: "Accept invitation", URL: self.cfg.PublicURL},
		Note:   "This invitation expires in 14 days. If you weren't expecting it, you can ignore this email.",
		Footer: mail.Footer{InstanceURL: self.cfg.PublicURL},
	})
}

// Sends is whether emails go out.
func (self *EmailService) Sends() bool {
	return self.mailer != nil
}

// SendSignInLink emails the link now, not through the queue, which would store its token.
func (self *EmailService) SendSignInLink(ctx context.Context, to, link string, ttl time.Duration) error {
	if self.mailer == nil {
		return errors.New("email is off")
	}
	message, err := mail.Render(to, mail.Content{
		Subject: "Sign in to Ricrac",
		Heading: "Sign in to Ricrac",
		Intro:   []string{fmt.Sprintf("Use this link to sign in as %s.", to)},
		Button:  mail.Button{Label: "Sign in", URL: link},
		Note:    fmt.Sprintf("The link works once and expires in %d minutes. If you didn't ask for it, you can ignore this email.", int(ttl.Minutes())),
		Footer:  mail.Footer{InstanceURL: self.cfg.PublicURL},
	})
	if err != nil {
		return err
	}
	return self.mailer.Send(ctx, message)
}

// SendEmails gathers due notification digests and sends the queued emails. Without a mailer it
// does nothing, and notifications stay unemailed.
func (self *EmailService) SendEmails(ctx context.Context) error {
	if self.mailer == nil {
		return nil
	}
	if err := self.queueDigests(ctx); err != nil {
		return err
	}
	return self.sendQueued(ctx)
}

func (self *EmailService) queueDigests(ctx context.Context) error {
	return self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		notifications, err := self.repo.Email().PendingNotifications(ctx, tx, time.Now().Add(-digestMaxAge))
		if err != nil || len(notifications) == 0 {
			return err
		}
		var users []uuid.UUID
		batches := map[uuid.UUID][]notice{}
		for _, n := range notifications {
			pending := noticeOf(n, self.cfg.PublicURL)
			if _, seen := batches[pending.UserID]; !seen {
				users = append(users, pending.UserID)
			}
			batches[pending.UserID] = append(batches[pending.UserID], pending)
		}
		lastEmailed, err := self.repo.Email().LastEmailed(ctx, tx, users)
		if err != nil {
			return err
		}
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		var emailed []uuid.UUID
		for _, user := range users {
			batch := batches[user]
			if !digestDue(batch[0].CreatedAt, lastEmailed[user], now) {
				continue
			}
			content, to := digest(batch, self.cfg.PublicURL)
			if err := self.queue(ctx, tx, to, content); err != nil {
				return err
			}
			for _, n := range batch {
				emailed = append(emailed, n.ID)
			}
		}
		if len(emailed) == 0 {
			return nil
		}
		return self.repo.Email().MarkEmailed(ctx, tx, emailed, now)
	})
}

func digestDue(oldest, lastEmailed, now time.Time) bool {
	return now.Sub(oldest) >= digestDelay && now.Sub(lastEmailed) >= digestInterval
}

func (self *EmailService) sendQueued(ctx context.Context) error {
	due, err := self.repo.Email().Due(ctx, sendAttempts)
	if err != nil {
		return err
	}
	for _, email := range due {
		var content mail.Content
		err := json.Unmarshal(email.Content, &content)
		var message mail.Message
		if err == nil {
			message, err = mail.Render(email.Recipient, content)
		}
		if err == nil {
			err = self.mailer.Send(ctx, message)
		}
		now, nowErr := self.repo.Now(ctx, nil)
		if nowErr != nil {
			return nowErr
		}
		if err == nil {
			if err := self.repo.Email().MarkSent(ctx, email.ID, now); err != nil {
				return err
			}
			continue
		}
		slog.Warn("email failed", "to", email.Recipient, "attempt", email.Attempts+1, "err", err)
		backoff := time.Duration(1<<(2*email.Attempts)) * time.Minute
		if err := self.repo.Email().MarkFailed(ctx, email.ID, err.Error(), now.Add(backoff)); err != nil {
			return err
		}
	}
	return nil
}
