package account_repo

import (
	"context"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/signinlink"
	"ricrac/ent/user"
	repository "ricrac/internal/repositories"
)

// LiveSignInLinks drops expired sign-in links and counts the email's others.
func (self *AccountRepository) LiveSignInLinks(ctx context.Context, tx repository.TxInterface, email string) (int, error) {
	client := self.base.Client(tx)
	_, err := client.SignInLink.Delete().
		Where(func(s *entsql.Selector) { s.Where(repository.Raw(s.C(signinlink.FieldExpiresAt) + " <= now()")) }).
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	return client.SignInLink.Query().Where(signinlink.Email(email)).Count(ctx)
}

func (self *AccountRepository) AddSignInLink(ctx context.Context, tx repository.TxInterface, tokenHash, email, returnTo string, expiresAt time.Time) error {
	return self.base.Client(tx).SignInLink.Create().
		SetID(tokenHash).SetEmail(email).SetReturnTo(returnTo).SetExpiresAt(expiresAt).
		Exec(ctx)
}

// TakeSignInLink uses up the email's unexpired link and answers where it goes, nil when there's none.
func (self *AccountRepository) TakeSignInLink(ctx context.Context, tx repository.TxInterface, tokenHash, email string) (*string, error) {
	client := self.base.Client(tx)
	link, err := client.SignInLink.Query().
		Where(signinlink.ID(tokenHash), signinlink.Email(email), future(signinlink.FieldExpiresAt)).
		ForUpdate().
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := client.SignInLink.DeleteOneID(tokenHash).Exec(ctx); err != nil {
		return nil, err
	}
	return &link.ReturnTo, nil
}

// UpsertEmailUser refreshes the user with this email, or creates them, and returns their id.
func (self *AccountRepository) UpsertEmailUser(ctx context.Context, tx repository.TxInterface, email, name, displayName string, now time.Time) (uuid.UUID, error) {
	return self.base.Client(tx).User.Create().
		SetEmail(email).SetName(name).SetDisplayName(displayName).
		OnConflictColumns(user.FieldEmail).
		Update(func(u *ent.UserUpsert) { u.SetLastSeenAt(now) }).
		ID(ctx)
}
