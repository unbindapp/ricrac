package team_repo

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/schema/pgarray"
	"ricrac/ent/team"
	"ricrac/ent/teammember"
	"ricrac/ent/workflowstate"
	repository "ricrac/internal/repositories"
)

// Update changes the team as set says.
func (self *TeamRepository) Update(ctx context.Context, tx repository.TxInterface, id uuid.UUID, set func(*ent.TeamUpdateOne)) error {
	update := self.base.Client(tx).Team.UpdateOneID(id)
	set(update)
	return update.Exec(ctx)
}

// RenameIssues keeps every issue's identifier under the old key among its previous identifiers.
func (self *TeamRepository) RenameIssues(ctx context.Context, tx repository.TxInterface, id uuid.UUID, oldKey string) error {
	_, err := self.base.Client(tx).ExecContext(ctx,
		`UPDATE issues SET previous_identifiers = array_append(previous_identifiers, $2 || '-' || number)
		 WHERE team_id = $1 AND NOT ($2 || '-' || number) = ANY(previous_identifiers)`, id, oldKey)
	return err
}

// AddMembers adds the users to the team, skipping those already in it.
func (self *TeamRepository) AddMembers(ctx context.Context, tx repository.TxInterface, id uuid.UUID, users []uuid.UUID) error {
	if len(users) == 0 {
		return nil
	}
	client := self.base.Client(tx)
	builders := make([]*ent.TeamMemberCreate, len(users))
	for i, user := range users {
		builders[i] = client.TeamMember.Create().SetTeamID(id).SetUserID(user)
	}
	return client.TeamMember.CreateBulk(builders...).
		OnConflictColumns(teammember.FieldTeamID, teammember.FieldUserID).DoNothing().Exec(ctx)
}

func (self *TeamRepository) RemoveMembers(ctx context.Context, tx repository.TxInterface, id uuid.UUID, users []uuid.UUID) error {
	_, err := self.base.Client(tx).TeamMember.Delete().Where(teammember.TeamID(id), teammember.UserIDIn(users...)).Exec(ctx)
	return err
}

// DropFromTriage takes the users off the team's triage, keeping the others' order.
func (self *TeamRepository) DropFromTriage(ctx context.Context, tx repository.TxInterface, id uuid.UUID, users []uuid.UUID) error {
	found, err := self.base.Client(tx).Team.Get(ctx, id)
	if err != nil {
		return err
	}
	kept := pgarray.UUID{}
	for _, person := range found.TriageResponsibleIds {
		if !slices.Contains(users, person) {
			kept = append(kept, person)
		}
	}
	return self.base.Client(tx).Team.UpdateOneID(id).SetTriageResponsibleIds(kept).Exec(ctx)
}

// ArchiveTriageStates archives the team's active triage states and says which they were.
func (self *TeamRepository) ArchiveTriageStates(ctx context.Context, tx repository.TxInterface, id uuid.UUID, now time.Time) ([]uuid.UUID, error) {
	client := self.base.Client(tx)
	ids, err := client.WorkflowState.Query().
		Where(workflowstate.TeamID(id), workflowstate.Type("triage"), workflowstate.ArchivedAtIsNil()).IDs(ctx)
	if err != nil || len(ids) == 0 {
		return ids, err
	}
	_, err = client.WorkflowState.Update().Where(workflowstate.IDIn(ids...)).SetArchivedAt(now).Save(ctx)
	return ids, err
}

func (self *TeamRepository) Unarchive(ctx context.Context, tx repository.TxInterface, state uuid.UUID) error {
	return self.base.Client(tx).WorkflowState.UpdateOneID(state).ClearArchivedAt().Exec(ctx)
}

// Archive archives the team and frees its sub-teams.
func (self *TeamRepository) Archive(ctx context.Context, tx repository.TxInterface, id uuid.UUID, now time.Time) (*ent.Team, error) {
	client := self.base.Client(tx)
	archived, err := client.Team.UpdateOneID(id).SetArchivedAt(now).SetUpdatedAt(now).Save(ctx)
	if err != nil {
		return nil, err
	}
	_, err = client.Team.Update().Where(team.ParentID(id)).ClearParentID().Save(ctx)
	return archived, err
}

// Join adds the user to the team unless they're in it.
func (self *TeamRepository) Join(ctx context.Context, tx repository.TxInterface, id, user uuid.UUID) error {
	return self.AddMembers(ctx, tx, id, []uuid.UUID{user})
}
