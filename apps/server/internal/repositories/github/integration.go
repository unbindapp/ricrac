package github_repo

import (
	"context"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/githubinstallation"
	"ricrac/ent/githubissuelink"
	"ricrac/ent/githubreposync"
	repository "ricrac/internal/repositories"
)

// Installations is the workspace's installations, oldest first, with their installer.
func (self *GitHubRepository) Installations(ctx context.Context, workspace uuid.UUID) ([]*ent.GithubInstallation, error) {
	return self.base.DB.GithubInstallation.Query().Where(githubinstallation.WorkspaceID(workspace)).
		WithInstaller().Order(githubinstallation.ByCreatedAt()).All(ctx)
}

// ActiveInstallations is the ids of the workspace's installations that aren't suspended.
func (self *GitHubRepository) ActiveInstallations(ctx context.Context, workspace uuid.UUID) ([]int64, error) {
	return self.base.DB.GithubInstallation.Query().
		Where(githubinstallation.WorkspaceID(workspace), githubinstallation.SuspendedAtIsNil()).
		Order(githubinstallation.ByCreatedAt()).IDs(ctx)
}

func (self *GitHubRepository) DeleteInstallation(ctx context.Context, tx repository.TxInterface, id int64) error {
	_, err := self.base.Client(tx).GithubInstallation.Delete().Where(githubinstallation.ID(id)).Exec(ctx)
	return err
}

func repoSyncsOf(client *ent.Client, workspace uuid.UUID, teams []uuid.UUID) *ent.GithubRepoSyncQuery {
	return client.GithubRepoSync.Query().
		Where(githubreposync.WorkspaceID(workspace), githubreposync.TeamIDIn(teams...)).
		WithTeam().WithProject().
		Order(func(s *entsql.Selector) {
			s.OrderExpr(entsql.Expr("lower(" + s.C(githubreposync.FieldFullName) + ")"))
		})
}

// RepoSyncs is the workspace's repository syncs with these teams, by repository name.
func (self *GitHubRepository) RepoSyncs(ctx context.Context, workspace uuid.UUID, teams []uuid.UUID) ([]*ent.GithubRepoSync, error) {
	return repoSyncsOf(self.base.DB, workspace, teams).All(ctx)
}

// RepoSyncPage is a page of RepoSyncs.
func (self *GitHubRepository) RepoSyncPage(ctx context.Context, workspace uuid.UUID, teams []uuid.UUID, limit, offset int) ([]*ent.GithubRepoSync, error) {
	return repoSyncsOf(self.base.DB, workspace, teams).Order(githubreposync.ByID()).Limit(limit).Offset(offset).All(ctx)
}

// RepoSync is a sync with its team and project, nil when there's none.
func (self *GitHubRepository) RepoSync(ctx context.Context, id uuid.UUID) (*ent.GithubRepoSync, error) {
	found, err := self.base.DB.GithubRepoSync.Query().Where(githubreposync.ID(id)).WithTeam().WithProject().Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

// LinkedIssueCounts is how many issues each sync links.
func (self *GitHubRepository) LinkedIssueCounts(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	var rows []struct {
		RepoSyncID uuid.UUID `json:"repo_sync_id"`
		Count      int       `json:"count"`
	}
	err := self.base.DB.GithubIssueLink.Query().Where(githubissuelink.RepoSyncIDIn(ids...)).
		GroupBy(githubissuelink.FieldRepoSyncID).Aggregate(ent.Count()).Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	counts := make(map[uuid.UUID]int, len(rows))
	for _, row := range rows {
		counts[row.RepoSyncID] = row.Count
	}
	return counts, nil
}

// SyncedRepos is the workspace's syncs, with only their repository and id.
func (self *GitHubRepository) SyncedRepos(ctx context.Context, workspace uuid.UUID) ([]*ent.GithubRepoSync, error) {
	return self.base.DB.GithubRepoSync.Query().Where(githubreposync.WorkspaceID(workspace)).
		Select(githubreposync.FieldRepoID).All(ctx)
}

// FindRepoSync is a sync of the workspace by id or repository full name.
func (self *GitHubRepository) FindRepoSync(ctx context.Context, workspace uuid.UUID, value string) (uuid.UUID, bool, error) {
	id, err := self.base.DB.GithubRepoSync.Query().
		Where(githubreposync.WorkspaceID(workspace), func(s *entsql.Selector) {
			s.Where(repository.Raw(s.C(githubreposync.FieldID)+"::text = ? OR lower("+s.C(githubreposync.FieldFullName)+") = lower(?)", value, value))
		}).FirstID(ctx)
	if ent.IsNotFound(err) {
		return uuid.Nil, false, nil
	}
	return id, err == nil, err
}

// RepoTaken says whether some workspace syncs the repository already.
func (self *GitHubRepository) RepoTaken(ctx context.Context, repoID int64) (bool, error) {
	return self.base.DB.GithubRepoSync.Query().Where(githubreposync.RepoID(repoID)).Exist(ctx)
}

type NewRepoSync struct {
	Workspace    uuid.UUID
	Installation int64
	RepoID       int64
	FullName     string
	Team         uuid.UUID
	Project      *uuid.UUID
	LinkBack     bool
}

func (self *GitHubRepository) CreateRepoSync(ctx context.Context, input NewRepoSync) (uuid.UUID, error) {
	created, err := self.base.DB.GithubRepoSync.Create().
		SetWorkspaceID(input.Workspace).SetInstallationID(input.Installation).SetRepoID(input.RepoID).
		SetFullName(input.FullName).SetTeamID(input.Team).SetNillableProjectID(input.Project).SetLinkBack(input.LinkBack).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// RepoSyncChange is what changes on a sync; nil fields stay, the project only when SetProject.
type RepoSyncChange struct {
	Team       *uuid.UUID
	SetProject bool
	Project    *uuid.UUID
	LinkBack   *bool
}

func (self *GitHubRepository) ChangeRepoSync(ctx context.Context, id uuid.UUID, change RepoSyncChange) error {
	update := self.base.DB.GithubRepoSync.Update().Where(githubreposync.ID(id)).SetNillableTeamID(change.Team).
		SetNillableLinkBack(change.LinkBack).
		Modify(func(u *entsql.UpdateBuilder) { u.Set(githubreposync.FieldUpdatedAt, now()) })
	if change.SetProject {
		update.SetNillableProjectID(change.Project)
		if change.Project == nil {
			update.ClearProjectID()
		}
	}
	return update.Exec(ctx)
}

// RequestBackfill sets the sync's import of existing issues to start over.
func (self *GitHubRepository) RequestBackfill(ctx context.Context, id uuid.UUID, includeClosed *bool) error {
	return self.base.DB.GithubRepoSync.Update().Where(githubreposync.ID(id)).
		SetBackfillStatus("pending").SetBackfillPage(1).SetNillableBackfillClosed(includeClosed).
		SetIssuesCreated(0).SetIssuesUpdated(0).SetCommentsCreated(0).SetSkipped(0).Exec(ctx)
}

func (self *GitHubRepository) DeleteRepoSync(ctx context.Context, id uuid.UUID) error {
	_, err := self.base.DB.GithubRepoSync.Delete().Where(githubreposync.ID(id)).Exec(ctx)
	return err
}

// SaveInstallation stores an installation for the workspace, or brings it back up to date;
// false when another workspace has it.
func (self *GitHubRepository) SaveInstallation(ctx context.Context, id int64, workspace uuid.UUID, login, kind string, installer uuid.UUID) (bool, error) {
	rows, err := self.base.DB.QueryContext(ctx,
		`INSERT INTO github_installations (id, workspace_id, account_login, account_type, installer_id) VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (id) DO UPDATE SET account_login = excluded.account_login, account_type = excluded.account_type,
			suspended_at = NULL, updated_at = now()
		 WHERE github_installations.workspace_id = excluded.workspace_id
		 RETURNING true`, id, workspace, login, kind, installer)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	saved := false
	if rows.Next() {
		err = rows.Scan(&saved)
	}
	if err == nil {
		err = rows.Err()
	}
	return saved, err
}
