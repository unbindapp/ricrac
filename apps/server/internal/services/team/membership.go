package team_service

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
)

func (self *TeamService) Delete(ctx context.Context, actor *auth.Actor, input *IDInput) (models.DeletedTeam, error) {
	if err := actor.RequireMember(); err != nil {
		return models.DeletedTeam{}, err
	}
	team, err := self.resolve.TeamOrDefault(ctx, actor, input.ID)
	if err != nil {
		return models.DeletedTeam{}, err
	}
	var key string
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		only, err := self.isWorkspaceTeam(ctx, tx, actor, team)
		if err != nil {
			return err
		}
		if only {
			return errdefs.BadRequest("This is the workspace's team, which holds its issues, statuses and labels; it can't be deleted. Change it with save_team, or delete the workspace")
		}
		if actor.Role < auth.Admin {
			m, err := self.repo.Team().Membership(ctx, tx, team, actor.User)
			if err != nil {
				return err
			}
			if !m.Owner {
				return errdefs.Forbidden("Only workspace admins and the team's owners can delete a team")
			}
		}
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		archived, err := self.repo.Team().Archive(ctx, tx, team, now)
		if err != nil {
			return err
		}
		key = archived.Key
		if err := self.events.Audit(ctx, tx, actor, "team.archived", M{"team": team, "key": archived.Key, "name": archived.Name}); err != nil {
			return err
		}
		value, err := self.Value(ctx, tx, actor, team)
		if err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "Team", "remove", value, nil, &team)
	})
	if err != nil {
		return models.DeletedTeam{}, err
	}
	return models.DeletedTeam{Deleted: models.ID(team), Key: key}, nil
}

func (self *TeamService) Join(ctx context.Context, actor *auth.Actor, input *IDInput) (models.TeamDetail, error) {
	if err := actor.RequireMember(); err != nil {
		return models.TeamDetail{}, err
	}
	team, err := self.findTeam(ctx, actor, input.ID)
	if err != nil {
		return models.TeamDetail{}, err
	}
	var value models.TeamDetail
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		m, err := self.repo.Team().Membership(ctx, tx, team, actor.User)
		if err != nil {
			return err
		}
		if m.Private && !m.Member && actor.Role < auth.Admin {
			return errdefs.Forbidden("Only workspace admins can join a private team; ask a member to add you")
		}
		if err := self.repo.Team().Join(ctx, tx, team, actor.User); err != nil {
			return err
		}
		value, err = self.Value(ctx, tx, actor, team)
		return err
	})
	if err != nil {
		return models.TeamDetail{}, err
	}
	return value, nil
}

func (self *TeamService) Leave(ctx context.Context, actor *auth.Actor, input *IDInput) (models.LeftTeam, error) {
	team, err := self.resolve.TeamOrDefault(ctx, actor, input.ID)
	if err != nil {
		return models.LeftTeam{}, err
	}
	var left models.TeamRef
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		if err := self.repo.Team().Lock(ctx, tx, team); err != nil {
			return err
		}
		only, err := self.isWorkspaceTeam(ctx, tx, actor, team)
		if err != nil {
			return err
		}
		if only {
			return errdefs.BadRequest("Everyone in the workspace is on its team; leave the workspace instead")
		}
		m, err := self.repo.Team().Membership(ctx, tx, team, actor.User)
		if err != nil {
			return err
		}
		if !m.Member {
			return errdefs.BadRequest("You aren't a member of this team")
		}
		if err := self.repo.Team().RemoveMembers(ctx, tx, team, []uuid.UUID{actor.User}); err != nil {
			return err
		}
		if err := self.repo.Team().DropFromTriage(ctx, tx, team, []uuid.UUID{actor.User}); err != nil {
			return err
		}
		if err := self.check(ctx, tx, team); err != nil {
			if errdefs.As(err).Message == privateNeedsMember {
				return errdefs.BadRequest("You're the last member of this private team; add someone else or delete the team")
			}
			return err
		}
		t, err := self.repo.Team().Get(ctx, tx, team)
		if err != nil {
			return err
		}
		left = *models.TeamRefOf(t)
		return nil
	})
	if err != nil {
		return models.LeftTeam{}, err
	}
	return models.LeftTeam{Left: left}, nil
}

type TriageInput struct {
	ID    *string `json:"id" doc:"Optional: the workspace's team by default. Its key, name or id."`
	Limit *int    `json:"limit" doc:"How many upcoming shifts to list, 10 by default, at most 100."`
	User  *string `json:"user" doc:"Only this person's shifts: id, name, email or \"me\"."`
}

type shift struct {
	person       uuid.UUID
	starts, ends time.Time
}

// shifts is the rotation's shifts from the current one on.
func shifts(people []uuid.UUID, weeks int, start, today time.Time, limit int, only *uuid.UUID) []shift {
	if len(people) == 0 || weeks < 1 || only != nil && !slices.Contains(people, *only) {
		return nil
	}
	week := 7 * 24 * time.Hour
	turn := max(int(today.Sub(start)/week), 0) / weeks
	var found []shift
	for len(found) < limit {
		person := people[turn%len(people)]
		starts := start.Add(time.Duration(turn*weeks) * week)
		if only == nil || *only == person {
			found = append(found, shift{person, starts, starts.Add(time.Duration(weeks) * week)})
		}
		turn++
	}
	return found
}

func (self *TeamService) TriageResponsibility(ctx context.Context, actor *auth.Actor, input *TriageInput) (models.TriageResponsibility, error) {
	team, err := self.resolve.TeamOrDefault(ctx, actor, input.ID)
	if err != nil {
		return models.TriageResponsibility{}, err
	}
	var only *uuid.UUID
	if input.User != nil {
		user, err := self.resolve.User(ctx, actor, *input.User)
		if err != nil {
			return models.TriageResponsibility{}, err
		}
		only = &user
	}
	t, err := self.repo.Team().Get(ctx, nil, team)
	if err != nil {
		return models.TriageResponsibility{}, err
	}
	action, current, err := self.issues.TriageResponsible(ctx, nil, team)
	if err != nil {
		return models.TriageResponsibility{}, err
	}
	people := []uuid.UUID(t.TriageResponsibleIds)
	upcoming := []models.TriageShift{}
	if t.TriageRotationWeeks != nil && t.TriageRotationStart != nil {
		limit := 10
		if input.Limit != nil {
			limit = min(max(*input.Limit, 1), 100)
		}
		today := time.Now().UTC().Truncate(24 * time.Hour)
		for _, s := range shifts(people, *t.TriageRotationWeeks, *t.TriageRotationStart, today, limit, only) {
			users, err := self.repo.Team().Users(ctx, nil, []uuid.UUID{s.person})
			if err != nil {
				return models.TriageResponsibility{}, err
			}
			upcoming = append(upcoming, models.TriageShift{
				User: models.UserRefsOf(users)[0], StartsAt: models.Date(s.starts), EndsAt: models.Date(s.ends),
			})
		}
	}
	var rotation *models.TriageRotation
	if t.TriageRotationWeeks != nil {
		rotation = &models.TriageRotation{Weeks: *t.TriageRotationWeeks, Start: models.DatePtr(t.TriageRotationStart)}
	}
	responsible, err := self.repo.Team().Users(ctx, nil, current)
	if err != nil {
		return models.TriageResponsibility{}, err
	}
	everyone, err := self.repo.Team().Users(ctx, nil, people)
	if err != nil {
		return models.TriageResponsibility{}, err
	}
	return models.TriageResponsibility{
		Team: *models.TeamRefOf(t), TriageEnabled: t.TriageEnabled, Action: action,
		Responsible: models.UserRefsOf(responsible), Everyone: models.UserRefsOf(everyone),
		Rotation: rotation, UpcomingShifts: upcoming,
	}, nil
}
