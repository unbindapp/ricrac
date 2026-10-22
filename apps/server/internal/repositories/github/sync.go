package github_repo

import (
	"context"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/comment"
	"ricrac/ent/githubcommentlink"
	"ricrac/ent/githubinstallation"
	"ricrac/ent/githubissuelink"
	"ricrac/ent/githubreposync"
	"ricrac/ent/issuelabel"
	"ricrac/ent/issuelabellink"
	"ricrac/ent/team"
	"ricrac/ent/user"
	repository "ricrac/internal/repositories"
)

// SyncedRepo is the installation's sync of a repository, nil when there's none.
func (self *GitHubRepository) SyncedRepo(ctx context.Context, installation, repoID int64) (*ent.GithubRepoSync, error) {
	found, err := self.base.DB.GithubRepoSync.Query().
		Where(githubreposync.InstallationID(installation), githubreposync.RepoID(repoID)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

// SyncOf is a sync by id, nil when there's none.
func (self *GitHubRepository) SyncOf(ctx context.Context, id uuid.UUID) (*ent.GithubRepoSync, error) {
	found, err := self.base.DB.GithubRepoSync.Get(ctx, id)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

// IssueLink is the link of a GitHub issue of the sync, nil when it has none.
func (self *GitHubRepository) IssueLink(ctx context.Context, repoSync uuid.UUID, githubID int64) (*ent.GithubIssueLink, error) {
	found, err := self.base.DB.GithubIssueLink.Query().
		Where(githubissuelink.RepoSyncID(repoSync), githubissuelink.GithubID(githubID)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

func (self *GitHubRepository) AddIssueLink(ctx context.Context, tx repository.TxInterface, issueID, repoSync uuid.UUID, githubID int64, number int, nodeID, url string, updatedAt time.Time) error {
	return self.base.Client(tx).GithubIssueLink.Create().
		SetID(issueID).SetRepoSyncID(repoSync).SetGithubID(githubID).SetGithubNumber(number).
		SetGithubNodeID(nodeID).SetGithubURL(url).SetGithubUpdatedAt(updatedAt).Exec(ctx)
}

// TouchIssueLink notes the GitHub issue's last change, if it is later than the one known.
func (self *GitHubRepository) TouchIssueLink(ctx context.Context, tx repository.TxInterface, issueID uuid.UUID, updatedAt time.Time) error {
	return self.base.Client(tx).GithubIssueLink.Update().Where(githubissuelink.ID(issueID)).
		Modify(func(u *entsql.UpdateBuilder) {
			u.Set(githubissuelink.FieldGithubUpdatedAt,
				repository.RawExpr("greatest("+githubissuelink.FieldGithubUpdatedAt+", ?::timestamptz)", updatedAt))
		}).Exec(ctx)
}

func (self *GitHubRepository) DeleteIssueLink(ctx context.Context, issueID uuid.UUID) error {
	_, err := self.base.DB.GithubIssueLink.Delete().Where(githubissuelink.ID(issueID)).Exec(ctx)
	return err
}

// RecordSync notes whether the last change with GitHub worked: message is why it didn't.
func (self *GitHubRepository) RecordSync(ctx context.Context, id uuid.UUID, message *string) error {
	return self.base.DB.GithubRepoSync.Update().Where(githubreposync.ID(id)).SetNillableLastError(message).
		Modify(func(u *entsql.UpdateBuilder) {
			if message == nil {
				u.SetNull(githubreposync.FieldLastError)
			}
			u.Set(githubreposync.FieldLastSyncedAt, now())
		}).Exec(ctx)
}

// MemberByLogin is the active member of the workspace who linked this GitHub login.
func (self *GitHubRepository) MemberByLogin(ctx context.Context, workspace uuid.UUID, login string) (uuid.UUID, bool, error) {
	id, err := self.base.DB.User.Query().
		Where(user.GithubLogin(login), func(s *entsql.Selector) {
			s.Where(repository.Raw("EXISTS (SELECT 1 FROM workspace_members m WHERE m.user_id = "+s.C(user.FieldID)+
				" AND m.workspace_id = ? AND m.active)", workspace))
		}).FirstID(ctx)
	if ent.IsNotFound(err) {
		return uuid.Nil, false, nil
	}
	return id, err == nil, err
}

// CanSeeTeam says whether a user of the team's workspace sees its issues: a member of a public
// team's workspace who isn't a guest, or a member of the team.
func (self *GitHubRepository) CanSeeTeam(ctx context.Context, teamID, member uuid.UUID) (bool, error) {
	return self.base.DB.Team.Query().
		Where(team.ID(teamID), func(s *entsql.Selector) {
			id, private, workspace := s.C(team.FieldID), s.C(team.FieldPrivate), s.C(team.FieldWorkspaceID)
			s.Where(repository.Raw("EXISTS (SELECT 1 FROM workspace_members m WHERE m.workspace_id = "+workspace+" AND m.user_id = ? "+
				"AND ((NOT "+private+" AND m.role <> 'guest') OR EXISTS (SELECT 1 FROM team_members tm WHERE tm.team_id = "+id+
				" AND tm.user_id = ?)))", member, member))
		}).Exist(ctx)
}

func (self *GitHubRepository) TriageEnabled(ctx context.Context, teamID uuid.UUID) (bool, error) {
	found, err := self.base.DB.Team.Query().Where(team.ID(teamID)).Select(team.FieldTriageEnabled).Only(ctx)
	if err != nil {
		return false, err
	}
	return found.TriageEnabled, nil
}

// LabelNamed is the team's or workspace's label (not a group, not retired) with this name: the
// team's first, then labels outside groups, then the oldest.
func (self *GitHubRepository) LabelNamed(ctx context.Context, workspace, teamID uuid.UUID, name string) (uuid.UUID, bool, error) {
	id, err := self.base.DB.IssueLabel.Query().
		Where(issuelabel.WorkspaceID(workspace), issuelabel.Or(issuelabel.TeamIDIsNil(), issuelabel.TeamID(teamID)),
			issuelabel.IsGroup(false), issuelabel.RetiredAtIsNil(), func(s *entsql.Selector) {
				s.Where(repository.Raw("lower("+s.C(issuelabel.FieldName)+") = lower(?)", name))
			}).
		Order(func(s *entsql.Selector) {
			s.OrderExpr(entsql.Expr(s.C(issuelabel.FieldTeamID) + " IS NULL"))
			s.OrderExpr(entsql.Expr(s.C(issuelabel.FieldParentID) + " IS NOT NULL"))
		}, issuelabel.ByCreatedAt()).FirstID(ctx)
	if ent.IsNotFound(err) {
		return uuid.Nil, false, nil
	}
	return id, err == nil, err
}

// CreateLabel adds a workspace label nobody made.
func (self *GitHubRepository) CreateLabel(ctx context.Context, tx repository.TxInterface, workspace uuid.UUID, name, color string) (uuid.UUID, error) {
	created, err := self.base.Client(tx).IssueLabel.Create().SetWorkspaceID(workspace).SetName(name).SetColor(color).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

func (self *GitHubRepository) HasLabel(ctx context.Context, issueID, label uuid.UUID) (bool, error) {
	return self.base.DB.IssueLabelLink.Query().
		Where(issuelabellink.IssueID(issueID), issuelabellink.LabelID(label)).Exist(ctx)
}

// LabelsNamedOn is the issue's labels with this name.
func (self *GitHubRepository) LabelsNamedOn(ctx context.Context, issueID uuid.UUID, name string) ([]uuid.UUID, error) {
	return self.base.DB.IssueLabel.Query().
		Where(issuelabel.HasIssueLabelLinksWith(issuelabellink.IssueID(issueID)), func(s *entsql.Selector) {
			s.Where(repository.Raw("lower("+s.C(issuelabel.FieldName)+") = lower(?)", name))
		}).IDs(ctx)
}

// CommentLinked says whether a GitHub comment is linked to a comment already.
func (self *GitHubRepository) CommentLinked(ctx context.Context, tx repository.TxInterface, githubID int64) (bool, error) {
	return self.base.Client(tx).GithubCommentLink.Query().Where(githubcommentlink.GithubID(githubID)).Exist(ctx)
}

// LinkedComment is the comment a GitHub comment is linked to.
func (self *GitHubRepository) LinkedComment(ctx context.Context, githubID int64) (uuid.UUID, bool, error) {
	id, err := self.base.DB.GithubCommentLink.Query().Where(githubcommentlink.GithubID(githubID)).FirstID(ctx)
	if ent.IsNotFound(err) {
		return uuid.Nil, false, nil
	}
	return id, err == nil, err
}

func (self *GitHubRepository) AddCommentLink(ctx context.Context, tx repository.TxInterface, commentID, issueID uuid.UUID, githubID int64) error {
	return self.base.Client(tx).GithubCommentLink.Create().SetID(commentID).SetIssueID(issueID).SetGithubID(githubID).Exec(ctx)
}

// DateComment sets when a comment was written.
func (self *GitHubRepository) DateComment(ctx context.Context, tx repository.TxInterface, id uuid.UUID, at time.Time) error {
	return self.base.Client(tx).Comment.Update().Where(comment.ID(id)).SetCreatedAt(at).SetUpdatedAt(at).Exec(ctx)
}

// DueBackfills is up to five syncs waiting to import issues, of installations that aren't
// suspended, least recently touched first.
func (self *GitHubRepository) DueBackfills(ctx context.Context) ([]uuid.UUID, error) {
	return self.base.DB.GithubRepoSync.Query().
		Where(githubreposync.BackfillStatusIn("pending", "running"),
			githubreposync.HasInstallationWith(githubinstallation.SuspendedAtIsNil())).
		Order(githubreposync.ByUpdatedAt()).Limit(5).IDs(ctx)
}

func (self *GitHubRepository) FailBackfill(ctx context.Context, id uuid.UUID) error {
	return self.base.DB.GithubRepoSync.Update().Where(githubreposync.ID(id)).SetBackfillStatus("failed").
		Modify(func(u *entsql.UpdateBuilder) { u.Set(githubreposync.FieldUpdatedAt, now()) }).Exec(ctx)
}

// BackfillProgress is what a page of the import did.
type BackfillProgress struct {
	Status                              string
	NextPage                            int
	Created, Updated, Comments, Skipped int
}

func (self *GitHubRepository) AdvanceBackfill(ctx context.Context, id uuid.UUID, progress BackfillProgress) error {
	return self.base.DB.GithubRepoSync.Update().Where(githubreposync.ID(id)).
		SetBackfillStatus(progress.Status).SetBackfillPage(progress.NextPage).
		AddIssuesCreated(progress.Created).AddIssuesUpdated(progress.Updated).
		AddCommentsCreated(progress.Comments).AddSkipped(progress.Skipped).
		Modify(func(u *entsql.UpdateBuilder) { u.Set(githubreposync.FieldUpdatedAt, now()) }).Exec(ctx)
}
