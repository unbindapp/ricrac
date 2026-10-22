package github_service

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"ricrac/internal/integrations/github"
	github_repo "ricrac/internal/repositories/github"
)

func (self *GitHubService) seal(token github.UserToken) github_repo.SealedToken {
	sealed := github_repo.SealedToken{AccessToken: self.keys.Seal(token.AccessToken), ExpiresAt: token.ExpiresAt, RefreshExpires: token.RefreshExpires}
	if token.RefreshToken != nil {
		sealed.RefreshToken = new(self.keys.Seal(*token.RefreshToken))
	}
	return sealed
}

// userToken is the GitHub token a user linked, refreshed when it is about to expire; empty when
// they have none that works.
func (self *GitHubService) userToken(ctx context.Context, user uuid.UUID) (string, error) {
	sealed, err := self.github().UserToken(ctx, user)
	if err != nil || sealed == nil {
		return "", err
	}
	access, ok := self.keys.Open(sealed.AccessToken)
	if !ok {
		return "", self.github().DeleteUserToken(ctx, nil, user)
	}
	if sealed.ExpiresAt == nil || time.Until(*sealed.ExpiresAt) > 5*time.Minute {
		return access, nil
	}
	refresh, ok := "", false
	if sealed.RefreshToken != nil && (sealed.RefreshExpires == nil || time.Now().Before(*sealed.RefreshExpires)) {
		refresh, ok = self.keys.Open(*sealed.RefreshToken)
	}
	if !ok {
		return "", self.github().DeleteUserToken(ctx, nil, user)
	}
	fresh, err := self.app.RefreshUserToken(ctx, refresh)
	if errors.Is(err, github.ErrTokenRefused) {
		return "", self.github().DeleteUserToken(ctx, nil, user)
	}
	if err != nil {
		return "", err
	}
	return fresh.AccessToken, self.github().SaveUserToken(ctx, nil, user, self.seal(fresh))
}

// asAuthor runs call with the GitHub token of user, answering who it acted as: nil when user has
// no token that works or GitHub refuses them, for the app to act as itself instead.
func (self *GitHubService) asAuthor(ctx context.Context, user *uuid.UUID, call func(token string) error) (*uuid.UUID, error) {
	if user == nil {
		return nil, nil
	}
	token, err := self.userToken(ctx, *user)
	if err != nil || token == "" {
		return nil, err
	}
	err = call(token)
	var apiErr *github.APIError
	if !errors.As(err, &apiErr) {
		if err != nil {
			return nil, err
		}
		return user, nil
	}
	switch apiErr.Status {
	case http.StatusUnauthorized:
		return nil, self.github().DeleteUserToken(ctx, nil, *user)
	case http.StatusForbidden, http.StatusNotFound:
		return nil, nil
	}
	return nil, err
}
