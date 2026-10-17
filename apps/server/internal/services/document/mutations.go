package document_service

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	document_repo "ricrac/internal/repositories/document"
	events_service "ricrac/internal/services/events"
)

func (self *DocumentService) Save(ctx context.Context, actor *auth.Actor, input *SaveInput) (models.DocumentDetail, error) {
	var value models.DocumentDetail
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var id uuid.UUID
		var err error
		action := "create"
		if input.ID != nil {
			action = "update"
			if id, err = self.find(ctx, actor, *input.ID, false); err != nil {
				return err
			}
			err = self.update(ctx, tx, actor, id, input)
		} else {
			id, err = self.create(ctx, tx, actor, input)
		}
		if err != nil {
			return err
		}
		if value, err = self.value(ctx, tx, id); err != nil {
			return err
		}
		team, err := self.repo.Document().WebhookTeam(ctx, tx, id)
		if err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "Document", action, value, nil, team)
	})
	return value, err
}

// parentOf is the one parent named in the arguments, or nil when none is.
func (self *DocumentService) parentOf(ctx context.Context, actor *auth.Actor, input *SaveInput) (*document_repo.Parents, error) {
	team, err := self.resolve.OptionalTeam(ctx, actor, input.Team)
	if err != nil {
		return nil, err
	}
	named := []bool{
		input.Project != nil,
		input.Initiative != nil,
		input.Team != nil && input.Cycle == nil,
		input.Issue != nil,
		input.Cycle != nil,
		input.Release != nil,
	}
	switch count := len(slices.DeleteFunc(named, func(n bool) bool { return !n })); {
	case count == 0:
		return nil, nil
	case count > 1:
		return nil, errdefs.BadRequest("A document belongs to one of project, initiative, team, issue, cycle or release; pass only one")
	}
	var parent document_repo.Parents
	var id uuid.UUID
	switch {
	case input.Project != nil:
		id, err = self.resolve.Project(ctx, actor, *input.Project)
		parent.Project = &id
	case input.Initiative != nil:
		id, err = self.resolve.Initiative(ctx, actor, *input.Initiative)
		parent.Initiative = &id
	case input.Issue != nil:
		id, err = self.resolve.Issue(ctx, actor, *input.Issue)
		parent.Issue = &id
	case input.Cycle != nil:
		id, err = self.resolve.Cycle(ctx, actor, team, *input.Cycle)
		parent.Cycle = &id
	case input.Release != nil:
		id, err = self.resolve.Release(ctx, actor, *input.Release)
		parent.Release = &id
	default:
		parent.Team = team
	}
	if err != nil {
		return nil, err
	}
	return &parent, nil
}

// optionalColor checks a color argument: nil when it is null or left out.
func optionalColor(color args.Nullable[string]) (*string, error) {
	if !color.Valid {
		return nil, nil
	}
	checked, err := utils.CheckColor(color.Value)
	if err != nil {
		return nil, err
	}
	return &checked, nil
}

func (self *DocumentService) create(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, input *SaveInput) (uuid.UUID, error) {
	parent, err := self.parentOf(ctx, actor, input)
	if err != nil {
		return uuid.Nil, err
	}
	if parent == nil {
		return uuid.Nil, errdefs.BadRequest("Say where the document belongs: one of project, initiative, team, issue, cycle or release")
	}
	if input.Patch != nil {
		return uuid.Nil, errdefs.BadRequest("patch only applies when updating; pass content")
	}
	if input.RestoreVersion != nil {
		return uuid.Nil, errdefs.BadRequest("restoreVersion only applies when updating")
	}
	title, content := input.Title, input.Content
	if input.Template != nil {
		if title, content, err = self.fromTemplate(ctx, tx, actor, *input.Template, title, content); err != nil {
			return uuid.Nil, err
		}
	}
	checkedTitle, err := utils.NonEmpty(utils.OrEmpty(title), "title")
	if err != nil {
		return uuid.Nil, err
	}
	color, err := optionalColor(input.Color)
	if err != nil {
		return uuid.Nil, err
	}
	id, err := self.repo.Document().Create(ctx, tx, document_repo.CreateInput{
		Workspace: actor.Workspace, SlugID: utils.SlugID(), Title: checkedTitle, Content: utils.OrEmpty(content),
		Icon: input.Icon.Ptr(), Color: color, Parents: *parent, Creator: actor.User,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return id, self.notifyMentions(ctx, tx, actor, id, checkedTitle, "", utils.OrEmpty(content))
}

// fromTemplate is the title and content with what the document template gives filled in.
func (self *DocumentService) fromTemplate(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, name string, title, content *string) (*string, *string, error) {
	template, err := self.resolve.Template(ctx, actor, "document", name)
	if err != nil {
		return nil, nil, err
	}
	raw, err := self.repo.Template().Data(ctx, tx, template)
	if err != nil {
		return nil, nil, err
	}
	var data models.M
	_ = json.Unmarshal(raw, &data)
	text := func(key string) *string {
		value, ok := data[key].(string)
		if !ok {
			return nil
		}
		return &value
	}
	if title == nil {
		title = text("title")
	}
	if utils.OrEmpty(content) == "" {
		content = text("content")
	}
	return title, content, nil
}

func (self *DocumentService) update(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, input *SaveInput) error {
	if input.Template != nil {
		return errdefs.BadRequest("Templates only apply when creating a document")
	}
	current, err := self.repo.Document().GetForUpdate(ctx, tx, id)
	if err != nil {
		return err
	}
	title := current.Title
	if input.Title != nil {
		if title, err = utils.NonEmpty(*input.Title, "title"); err != nil {
			return err
		}
	}
	content := current.Content
	replaced, err := args.NewContent(current.Content, input.Content, input.Patch)
	if err != nil {
		return err
	}
	if replaced != nil {
		content = *replaced
	}
	restoring := input.RestoreVersion != nil
	if restoring {
		if title, content, err = self.versionText(ctx, tx, id, *input.RestoreVersion, input); err != nil {
			return err
		}
	}
	if title != current.Title || content != current.Content {
		if err := self.keepVersion(ctx, tx, actor, id, current.Title, current.Content, current.UpdatedByID, restoring); err != nil {
			return err
		}
		if err := self.repo.Document().SetText(ctx, tx, id, title, content); err != nil {
			return err
		}
		if err := self.notifyMentions(ctx, tx, actor, id, title, current.Content, content); err != nil {
			return err
		}
	}
	if input.Icon.Set {
		if err := self.repo.Document().SetIcon(ctx, tx, id, input.Icon.Ptr()); err != nil {
			return err
		}
	}
	if input.Color.Set {
		color, err := optionalColor(input.Color)
		if err != nil {
			return err
		}
		if err := self.repo.Document().SetColor(ctx, tx, id, color); err != nil {
			return err
		}
	}
	parent, err := self.parentOf(ctx, actor, input)
	if err != nil {
		return err
	}
	if parent != nil {
		if err := self.repo.Document().SetParents(ctx, tx, id, *parent); err != nil {
			return err
		}
	}
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return err
	}
	return self.repo.Document().Touch(ctx, tx, id, now, actor.User)
}

// versionText is the title and content of an earlier version of the document, to restore.
func (self *DocumentService) versionText(ctx context.Context, tx repository.TxInterface, id uuid.UUID, version int64, input *SaveInput) (string, string, error) {
	if input.Title != nil || input.Content != nil || input.Patch != nil {
		return "", "", errdefs.BadRequest("restoreVersion brings back a version's title and content; don't pass title, content or patch with it")
	}
	found, err := self.repo.Document().Version(ctx, tx, id, version)
	if err != nil {
		return "", "", err
	}
	if found == nil {
		return "", "", errdefs.NotFound("No version %d of this document; list_document_versions lists them", version)
	}
	return found.Title, found.Content, nil
}

// keepVersion keeps the current title and content as a version, unless the caller wrote them and
// the last version is less than 10 minutes old: a burst of edits by one person makes one version.
// A restore always keeps one, so it can be undone.
func (self *DocumentService) keepVersion(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, title, content string, updatedBy *uuid.UUID, always bool) error {
	recent, err := self.repo.Document().HasRecentVersion(ctx, tx, id)
	if err != nil {
		return err
	}
	if !always && recent && updatedBy != nil && *updatedBy == actor.User {
		return nil
	}
	return self.repo.Document().AddVersion(ctx, tx, id, title, content, updatedBy)
}

func (self *DocumentService) notifyMentions(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, title, before, after string) error {
	previous, err := self.events.Mentions(ctx, actor, before)
	if err != nil {
		return err
	}
	mentioned, err := self.events.Mentions(ctx, actor, after)
	if err != nil {
		return err
	}
	mentioned = slices.DeleteFunc(mentioned, func(u uuid.UUID) bool { return slices.Contains(previous, u) })
	if len(mentioned) == 0 {
		return nil
	}
	return self.events.Notify(ctx, tx, actor, mentioned, events_service.Notice{Kind: "documentMention", Summary: "You were mentioned in " + title, Document: &id})
}

func (self *DocumentService) setTrashed(ctx context.Context, actor *auth.Actor, id uuid.UUID, trashed bool) (models.DocumentDetail, error) {
	var value models.DocumentDetail
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		if err := self.repo.Document().SetTrashed(ctx, tx, id, trashed, now, actor.User); err != nil {
			return err
		}
		if value, err = self.value(ctx, tx, id); err != nil {
			return err
		}
		team, err := self.repo.Document().WebhookTeam(ctx, tx, id)
		if err != nil {
			return err
		}
		action := "update"
		if trashed {
			action = "remove"
		}
		return self.events.Emit(ctx, tx, actor, "Document", action, value, nil, team)
	})
	return value, err
}

func (self *DocumentService) Delete(ctx context.Context, actor *auth.Actor, input *IDInput) (models.DocumentDetail, error) {
	id, err := self.find(ctx, actor, input.ID, false)
	if err != nil {
		return models.DocumentDetail{}, err
	}
	return self.setTrashed(ctx, actor, id, true)
}

func (self *DocumentService) Restore(ctx context.Context, actor *auth.Actor, input *IDInput) (models.DocumentDetail, error) {
	id, err := self.find(ctx, actor, input.ID, true)
	if err != nil {
		return models.DocumentDetail{}, err
	}
	return self.setTrashed(ctx, actor, id, false)
}
