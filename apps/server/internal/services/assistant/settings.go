package assistant_service

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
)

// manager checks the caller may turn the assistant on and pick its users: an owner or admin of
// the workspace listed in ASSISTANT_ADMINS.
func (self *AssistantService) manager(ctx context.Context, actor *auth.Actor) error {
	if self.cfg.Assistant == nil {
		return errdefs.NotFound("The assistant is off: set ANTHROPIC_API_KEY or CLAUDE_CODE_OAUTH_TOKEN on the server")
	}
	user, err := self.repo.Account().User(ctx, actor.User)
	if err != nil {
		return err
	}
	if user == nil || actor.Role < auth.Admin || !self.cfg.Assistant.Manages(user.Email) {
		return errdefs.Forbidden("Only workspace admins listed in the server's ASSISTANT_ADMINS manage Claude")
	}
	return nil
}

func (self *AssistantService) Settings(ctx context.Context, actor *auth.Actor) (models.AssistantSettings, error) {
	if err := self.manager(ctx, actor); err != nil {
		return models.AssistantSettings{}, err
	}
	return self.settings(ctx, nil, actor)
}

func (self *AssistantService) settings(ctx context.Context, tx repository.TxInterface, actor *auth.Actor) (models.AssistantSettings, error) {
	enabled, users, err := self.repo.Assistant().Settings(ctx, tx, actor.Workspace)
	if err != nil {
		return models.AssistantSettings{}, err
	}
	ids := make([]string, len(users))
	for i, user := range users {
		ids[i] = user.String()
	}
	slices.Sort(ids)
	return models.AssistantSettings{Enabled: enabled, UserIDs: ids}, nil
}

func (self *AssistantService) SaveSettings(ctx context.Context, actor *auth.Actor, input *AssistantSettingsInput) (models.AssistantSettings, error) {
	if err := self.manager(ctx, actor); err != nil {
		return models.AssistantSettings{}, err
	}
	users, err := userIDs(input.UserIDs)
	if err != nil {
		return models.AssistantSettings{}, err
	}
	var saved models.AssistantSettings
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		if input.Enabled != nil {
			if err := self.repo.Assistant().SetEnabled(ctx, tx, actor.Workspace, *input.Enabled); err != nil {
				return err
			}
		}
		if users != nil {
			picked, err := self.repo.Assistant().SetUsers(ctx, tx, actor.Workspace, users)
			if err != nil {
				return err
			}
			if picked != len(users) {
				return errdefs.BadRequest("Only members of this workspace who aren't guests can use Claude")
			}
		}
		saved, err = self.settings(ctx, tx, actor)
		if err != nil {
			return err
		}
		return self.events.Audit(ctx, tx, actor, "assistant.updated", models.M{"enabled": saved.Enabled, "userIds": saved.UserIDs})
	})
	return saved, err
}

// userIDs are the users picked, without repeats; nil when they're left out.
func userIDs(values []string) ([]uuid.UUID, error) {
	if values == nil {
		return nil, nil
	}
	users := []uuid.UUID{}
	for _, value := range values {
		id, err := uuid.Parse(value)
		if err != nil {
			return nil, errdefs.BadRequest("%q isn't a user id", value)
		}
		if !slices.Contains(users, id) {
			users = append(users, id)
		}
	}
	return users, nil
}
