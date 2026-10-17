package github_repo

import (
	"context"
	"encoding/json"
	"slices"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/attachment"
	"ricrac/ent/githubinstallation"
	"ricrac/ent/githubreposync"
	"ricrac/ent/issue"
	"ricrac/ent/team"
	"ricrac/ent/workflowstate"
	repository "ricrac/internal/repositories"
)

// RecordEvent notes an installation's last webhook event, answering its workspace; false when no
// workspace has the installation.
func (self *GitHubRepository) RecordEvent(ctx context.Context, installation int64, kind string) (uuid.UUID, bool, error) {
	count, err := self.base.DB.GithubInstallation.Update().Where(githubinstallation.ID(installation)).SetLastEvent(kind).
		Modify(func(u *entsql.UpdateBuilder) { u.Set(githubinstallation.FieldLastEventAt, now()) }).Save(ctx)
	if err != nil || count == 0 {
		return uuid.Nil, false, err
	}
	found, err := self.base.DB.GithubInstallation.Query().Where(githubinstallation.ID(installation)).
		Select(githubinstallation.FieldWorkspaceID).Only(ctx)
	if ent.IsNotFound(err) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	return found.WorkspaceID, true, nil
}

// RenameRepo follows a repository renamed on GitHub.
func (self *GitHubRepository) RenameRepo(ctx context.Context, installation, repoID int64, fullName string) error {
	return self.base.DB.GithubRepoSync.Update().
		Where(githubreposync.InstallationID(installation), githubreposync.RepoID(repoID), githubreposync.FullNameNEQ(fullName)).
		SetFullName(fullName).Exec(ctx)
}

func (self *GitHubRepository) SetSuspended(ctx context.Context, installation int64, suspended bool) error {
	return self.base.DB.GithubInstallation.Update().Where(githubinstallation.ID(installation)).
		Modify(func(u *entsql.UpdateBuilder) {
			if suspended {
				u.Set(githubinstallation.FieldSuspendedAt, now())
			} else {
				u.SetNull(githubinstallation.FieldSuspendedAt)
			}
			u.Set(githubinstallation.FieldUpdatedAt, now())
		}).Exec(ctx)
}

// UnsyncRepos stops syncing repositories an installation lost.
func (self *GitHubRepository) UnsyncRepos(ctx context.Context, installation int64, repoIDs []int64) error {
	_, err := self.base.DB.GithubRepoSync.Delete().
		Where(githubreposync.InstallationID(installation), githubreposync.RepoIDIn(repoIDs...)).Exec(ctx)
	return err
}

// IssuesNamed is the workspace's issues (not in the trash) with this team key and number, or that
// had this identifier before.
func (self *GitHubRepository) IssuesNamed(ctx context.Context, workspace uuid.UUID, key string, number int, identifier string) ([]uuid.UUID, error) {
	return self.base.DB.Issue.Query().
		Where(issue.WorkspaceID(workspace), issue.TrashedAtIsNil(), issue.Or(
			issue.And(issue.Number(number), issue.HasTeamWith(func(s *entsql.Selector) {
				s.Where(repository.Raw("upper("+s.C(team.FieldKey)+") = ?", key))
			})),
			func(s *entsql.Selector) {
				s.Where(repository.Raw("? = ANY("+s.C(issue.FieldPreviousIdentifiers)+")", identifier))
			},
		)).IDs(ctx)
}

// HasPullRequestFrom says whether the issue has a pull request from this branch attached.
func (self *GitHubRepository) HasPullRequestFrom(ctx context.Context, tx repository.TxInterface, id uuid.UUID, sourceType, repo, branch string) (bool, error) {
	return self.base.Client(tx).Attachment.Query().
		Where(attachment.IssueID(id), attachment.SourceType(sourceType), func(s *entsql.Selector) {
			metadata := s.C(attachment.FieldMetadata)
			s.Where(repository.Raw(metadata+"->>'repo' = ? AND "+metadata+"->>'branch' = ?", repo, branch))
		}).Exist(ctx)
}

// AttachmentsAt is the issue's attachments with these URLs, in their order.
func (self *GitHubRepository) AttachmentsAt(ctx context.Context, tx repository.TxInterface, id uuid.UUID, urls []string) ([]*ent.Attachment, error) {
	found, err := self.base.Client(tx).Attachment.Query().Where(attachment.IssueID(id), attachment.URLIn(urls...)).All(ctx)
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(found, func(a, b *ent.Attachment) int { return slices.Index(urls, a.URL) - slices.Index(urls, b.URL) })
	return found, nil
}

type NewAttachment struct {
	Workspace, Issue uuid.UUID
	URL, Title       string
	Metadata         json.RawMessage
	SourceType       string
}

func (self *GitHubRepository) AddAttachment(ctx context.Context, tx repository.TxInterface, input NewAttachment) (uuid.UUID, error) {
	created, err := self.base.Client(tx).Attachment.Create().
		SetWorkspaceID(input.Workspace).SetIssueID(input.Issue).SetURL(input.URL).SetTitle(input.Title).
		SetMetadata(input.Metadata).SetSourceType(input.SourceType).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

func (self *GitHubRepository) ChangeAttachment(ctx context.Context, tx repository.TxInterface, id uuid.UUID, input NewAttachment) error {
	return self.base.Client(tx).Attachment.Update().Where(attachment.ID(id)).
		SetURL(input.URL).SetTitle(input.Title).SetSourceType(input.SourceType).SetMetadata(input.Metadata).
		Modify(func(u *entsql.UpdateBuilder) { u.Set(attachment.FieldUpdatedAt, now()) }).Exec(ctx)
}

// PullRequestState is the team's unarchived status chosen in column, else its first of type
// fallback; nil when it has neither.
func (self *GitHubRepository) PullRequestState(ctx context.Context, tx repository.TxInterface, teamID uuid.UUID, merged bool, fallback string) (*uuid.UUID, error) {
	client := self.base.Client(tx)
	found, err := client.Team.Get(ctx, teamID)
	if err != nil {
		return nil, err
	}
	chosen := found.GithubPrOpenedStateID
	if merged {
		chosen = found.GithubPrMergedStateID
	}
	matches := workflowstate.Type(fallback)
	if chosen != nil {
		matches = workflowstate.Or(workflowstate.ID(*chosen), matches)
	}
	query := client.WorkflowState.Query().Where(workflowstate.TeamID(teamID), workflowstate.ArchivedAtIsNil(), matches)
	if chosen != nil {
		query.Order(func(s *entsql.Selector) {
			s.OrderExpr(repository.RawExpr(s.C(workflowstate.FieldID)+" = ? DESC", *chosen))
		})
	}
	id, err := query.Order(workflowstate.ByPosition()).FirstID(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}
