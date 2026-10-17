package label_service

import (
	"context"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	label_repo "ricrac/internal/repositories/label"
)

func (self *LabelService) List(ctx context.Context, actor *auth.Actor, input *ListInput) (args.PageOf[models.IssueLabel], error) {
	team, err := self.resolve.OptionalTeam(ctx, actor, input.Team)
	if err != nil {
		return args.PageOf[models.IssueLabel]{}, err
	}
	var teams []uuid.UUID
	if team != nil {
		teams = []uuid.UUID{*team}
	} else if teams, err = actor.TeamIDs(ctx); err != nil {
		return args.PageOf[models.IssueLabel]{}, err
	}
	offset, err := input.Offset()
	if err != nil {
		return args.PageOf[models.IssueLabel]{}, err
	}
	ids, err := self.repo.Label().List(ctx, label_repo.ListFilter{
		Workspace: actor.Workspace, Teams: teams, IncludeRetired: input.IncludeRetired,
		Pattern: utils.LikePattern(input.Query), Limit: input.Size() + 1, Offset: offset,
	})
	if err != nil {
		return args.PageOf[models.IssueLabel]{}, err
	}
	labels, err := self.values(ctx, nil, ids)
	if err != nil {
		return args.PageOf[models.IssueLabel]{}, err
	}
	return args.WrapOf(input.Page, labels)
}

func (self *LabelService) Get(ctx context.Context, actor *auth.Actor, input *LabelInput) (models.IssueLabel, error) {
	label, err := self.labelOf(ctx, actor, input)
	if err != nil {
		return models.IssueLabel{}, err
	}
	return self.value(ctx, nil, label)
}
