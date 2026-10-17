package cycle_service

import (
	"context"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/models"
	cycle_repo "ricrac/internal/repositories/cycle"
)

// singleKinds are the list types that answer one cycle at most.
var singleKinds = map[string]bool{"all": false, "current": true, "previous": true, "next": true, "upcoming": false, "completed": false}

func (self *CycleService) List(ctx context.Context, actor *auth.Actor, input *ListInput) (args.PageOf[models.Cycle], error) {
	team, err := self.resolve.Team(ctx, actor, input.Team)
	if err != nil {
		return args.PageOf[models.Cycle]{}, err
	}
	kind := "all"
	if input.Type != nil {
		kind = *input.Type
	}
	single, ok := singleKinds[kind]
	if !ok {
		return args.PageOf[models.Cycle]{}, errdefs.BadRequest("type %q isn't one of current, previous, next, upcoming, completed, all", kind)
	}
	offset, err := input.Offset()
	if err != nil {
		return args.PageOf[models.Cycle]{}, err
	}
	limit := input.Size() + 1
	if single {
		limit = 1
	}
	ids, err := self.repo.Cycle().List(ctx, cycle_repo.ListFilter{
		Team: team, Kind: kind, IncludeArchived: input.IncludeArchived, Limit: limit, Offset: offset,
	})
	if err != nil {
		return args.PageOf[models.Cycle]{}, err
	}
	cycles, err := self.values(ctx, nil, ids)
	if err != nil {
		return args.PageOf[models.Cycle]{}, err
	}
	return args.WrapOf(input.Page, cycles)
}

func (self *CycleService) Get(ctx context.Context, actor *auth.Actor, input *GetInput) (models.CycleDetail, error) {
	id, err := self.find(ctx, actor, input.Team, input.ID)
	if err != nil {
		return models.CycleDetail{}, err
	}
	value, err := self.value(ctx, nil, id)
	if err != nil {
		return models.CycleDetail{}, err
	}
	detail := models.CycleDetail{Cycle: value}
	if input.IncludeIssues != nil && !*input.IncludeIssues {
		return detail, nil
	}
	issues, err := self.repo.Cycle().Issues(ctx, nil, id)
	if err != nil {
		return models.CycleDetail{}, err
	}
	detail.Issues = make(models.CycleIssues, len(issues))
	for i, issue := range issues {
		detail.Issues[i] = models.CycleIssueOf(issue)
	}
	return detail, nil
}
