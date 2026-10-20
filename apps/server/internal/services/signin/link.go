package signin_service

import (
	"context"
	"log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	repository "ricrac/internal/repositories"
)

const (
	LinkTTL      = 15 * time.Minute
	maxLiveLinks = 3
)

// SendLink emails a link that signs in as email and then goes on to returnTo.
func (self *SigninService) SendLink(ctx context.Context, email, returnTo string) error {
	if !self.emails.Sends() {
		return errdefs.BadRequest("Signing in by email isn't set up on this server")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if parsed, err := mail.ParseAddress(email); err != nil || parsed.Address != email {
		return errdefs.BadRequest("Enter a valid email address")
	}
	token := auth.RandomSecret("")
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		live, err := self.repo.Account().LiveSignInLinks(ctx, tx, email)
		if err != nil {
			return err
		}
		if live >= maxLiveLinks {
			return errdefs.Errorf(http.StatusTooManyRequests, "Too many sign-in links went to %s. Use one of them, or try again in a few minutes", email)
		}
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		return self.repo.Account().AddSignInLink(ctx, tx, auth.SecretHash(token), email, SafeReturnTo(returnTo), now.Add(LinkTTL))
	})
	if err != nil {
		return err
	}
	link := self.cfg.PublicURL + "/sign-in/email?" + url.Values{"email": {email}, "token": {token}}.Encode()
	if err := self.emails.SendSignInLink(ctx, email, link, LinkTTL); err != nil {
		slog.Warn("sign-in email failed", "err", err)
		return errdefs.Errorf(http.StatusBadGateway, "The email couldn't be sent. Try again in a moment")
	}
	return nil
}

// FinishLink signs in with an emailed link, once: where to go next and who signed in.
func (self *SigninService) FinishLink(ctx context.Context, email, token string) (string, uuid.UUID, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var returnTo string
	var user uuid.UUID
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		taken, err := self.repo.Account().TakeSignInLink(ctx, tx, auth.SecretHash(token), email)
		if err != nil {
			return err
		}
		if taken == nil {
			return errdefs.BadRequest("This sign-in link expired or was already used. Ask for a new one")
		}
		returnTo = *taken
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		name, _, _ := strings.Cut(email, "@")
		user, err = self.repo.Account().UpsertEmailUser(ctx, tx, email, name, auth.DisplayName(name, email), now)
		return err
	})
	return returnTo, user, err
}
