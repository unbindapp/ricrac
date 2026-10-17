package template_service

import (
	"context"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/models"
	template_repo "ricrac/internal/repositories/template"
)

func (self *TemplateService) List(ctx context.Context, actor *auth.Actor, input *ListInput) (args.PageOf[models.Template], error) {
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return args.PageOf[models.Template]{}, err
	}
	filter := template_repo.ListFilter{Workspace: actor.Workspace, Teams: teams}
	if input.Type != nil {
		kind, err := checkType(*input.Type)
		if err != nil {
			return args.PageOf[models.Template]{}, err
		}
		filter.Type = &kind
	}
	if input.Team != nil {
		team, err := self.resolve.Team(ctx, actor, *input.Team)
		if err != nil {
			return args.PageOf[models.Template]{}, err
		}
		filter.Team = &team
	}
	if filter.Offset, err = input.Offset(); err != nil {
		return args.PageOf[models.Template]{}, err
	}
	filter.Limit = input.Size() + 1
	ids, err := self.repo.Template().List(ctx, filter)
	if err != nil {
		return args.PageOf[models.Template]{}, err
	}
	templates, err := self.repo.Template().Load(ctx, nil, ids)
	if err != nil {
		return args.PageOf[models.Template]{}, err
	}
	values := make([]models.Template, len(templates))
	for i, t := range templates {
		values[i] = models.TemplateOf(t)
	}
	return args.WrapOf(input.Page, values)
}

func (self *TemplateService) Get(ctx context.Context, actor *auth.Actor, input *GetInput) (models.TemplateDetail, error) {
	id, err := self.find(ctx, actor, input.Type, input.ID)
	if err != nil {
		return models.TemplateDetail{}, err
	}
	return self.value(ctx, nil, id)
}
