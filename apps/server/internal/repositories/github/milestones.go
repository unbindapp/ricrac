package github_repo

import (
	"context"
	"slices"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/githubinstallation"
	"ricrac/ent/githubissuelink"
	"ricrac/ent/githubmilestonelink"
	"ricrac/ent/githubreposync"
	team_ent "ricrac/ent/team"
	"ricrac/ent/user"
	"ricrac/ent/workspacemember"
	repository "ricrac/internal/repositories"
)

// MilestoneLink is the link of a GitHub milestone of the sync, nil when it has none.
func (self *GitHubRepository) MilestoneLink(ctx context.Context, repoSync uuid.UUID, githubID int64) (*ent.GithubMilestoneLink, error) {
	found, err := self.base.DB.GithubMilestoneLink.Query().
		Where(githubmilestonelink.RepoSyncID(repoSync), githubmilestonelink.GithubID(githubID)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

// MilestoneLinkIn is the link of a Ricrac milestone to the sync's repository, nil when it has none.
func (self *GitHubRepository) MilestoneLinkIn(ctx context.Context, milestone, repoSync uuid.UUID) (*ent.GithubMilestoneLink, error) {
	found, err := self.base.DB.GithubMilestoneLink.Query().
		Where(githubmilestonelink.MilestoneID(milestone), githubmilestonelink.RepoSyncID(repoSync)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

// MilestoneLinks is the links of the sync's repository.
func (self *GitHubRepository) MilestoneLinks(ctx context.Context, repoSync uuid.UUID) ([]*ent.GithubMilestoneLink, error) {
	return self.base.DB.GithubMilestoneLink.Query().Where(githubmilestonelink.RepoSyncID(repoSync)).All(ctx)
}

// OtherMilestoneLinks says whether the milestone is linked to a GitHub milestone but this one.
func (self *GitHubRepository) OtherMilestoneLinks(ctx context.Context, milestone, link uuid.UUID) (bool, error) {
	return self.base.DB.GithubMilestoneLink.Query().
		Where(githubmilestonelink.MilestoneID(milestone), githubmilestonelink.IDNEQ(link)).Exist(ctx)
}

func (self *GitHubRepository) AddMilestoneLink(ctx context.Context, milestone, repoSync uuid.UUID, githubID int64, number int, url string, updatedAt time.Time) error {
	return self.base.DB.GithubMilestoneLink.Create().SetMilestoneID(milestone).SetRepoSyncID(repoSync).
		SetGithubID(githubID).SetGithubNumber(number).SetGithubURL(url).SetGithubUpdatedAt(updatedAt).Exec(ctx)
}

// TouchMilestoneLink notes the GitHub milestone's last change, if it is later than the one known.
func (self *GitHubRepository) TouchMilestoneLink(ctx context.Context, id uuid.UUID, updatedAt time.Time) error {
	return self.base.DB.GithubMilestoneLink.Update().Where(githubmilestonelink.ID(id)).
		Modify(func(u *entsql.UpdateBuilder) {
			u.Set(githubmilestonelink.FieldGithubUpdatedAt,
				repository.RawExpr("greatest("+githubmilestonelink.FieldGithubUpdatedAt+", ?::timestamptz)", updatedAt))
		}).Exec(ctx)
}

func (self *GitHubRepository) DeleteMilestoneLink(ctx context.Context, id uuid.UUID) error {
	return self.base.DB.GithubMilestoneLink.DeleteOneID(id).Exec(ctx)
}

// MilestoneSyncsDue is up to limit syncs whose milestones weren't read from GitHub in the last ten
// minutes, of installations that aren't suspended, the longest waiting first.
func (self *GitHubRepository) MilestoneSyncsDue(ctx context.Context, limit int) ([]*ent.GithubRepoSync, error) {
	return self.base.DB.GithubRepoSync.Query().
		Where(githubreposync.HasInstallationWith(githubinstallation.SuspendedAtIsNil()), func(s *entsql.Selector) {
			synced := s.C(githubreposync.FieldMilestonesSyncedAt)
			s.Where(repository.Raw(synced + " IS NULL OR " + synced + " < now() - interval '10 minutes'"))
		}).
		Modify(func(s *entsql.Selector) {
			s.OrderExpr(entsql.Expr(s.C(githubreposync.FieldMilestonesSyncedAt) + " NULLS FIRST"))
		}).Limit(limit).All(ctx)
}

func (self *GitHubRepository) MarkMilestonesSynced(ctx context.Context, id uuid.UUID) error {
	return self.base.DB.GithubRepoSync.Update().Where(githubreposync.ID(id)).
		Modify(func(u *entsql.UpdateBuilder) { u.Set(githubreposync.FieldMilestonesSyncedAt, now()) }).Exec(ctx)
}

// TeamSyncsFields says whether the team syncs priority, estimate, due date and cycle with GitHub's
// issue fields.
func (self *GitHubRepository) TeamSyncsFields(ctx context.Context, team uuid.UUID) (bool, error) {
	return self.base.DB.Team.Query().Where(team_ent.ID(team), team_ent.GithubSyncFields(true)).Exist(ctx)
}

// MentionLogins is the GitHub login of each of handles naming an active member who linked GitHub:
// by display name, email or the part of their email before the @.
func (self *GitHubRepository) MentionLogins(ctx context.Context, workspace uuid.UUID, handles []string) (map[string]string, error) {
	members, err := self.base.DB.User.Query().
		Where(user.GithubLoginNotNil(), user.HasWorkspaceMembersWith(workspacemember.WorkspaceID(workspace), workspacemember.Active(true))).
		All(ctx)
	if err != nil {
		return nil, err
	}
	logins := map[string]string{}
	for _, member := range members {
		email := strings.ToLower(member.Email)
		local, _, _ := strings.Cut(email, "@")
		for _, name := range []string{strings.ToLower(member.DisplayName), email, local} {
			if slices.Contains(handles, name) {
				logins[name] = *member.GithubLogin
			}
		}
	}
	return logins, nil
}

// LockNewIssues waits for the sync's issues being opened on GitHub to be linked, until tx ends.
func (self *GitHubRepository) LockNewIssues(ctx context.Context, tx repository.TxInterface, repoSync uuid.UUID) error {
	_, err := self.base.Client(tx).ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended('new-issues:' || $1::text, 0))", repoSync)
	return err
}

// IssueLinked says whether the issue is linked to a GitHub issue.
func (self *GitHubRepository) IssueLinked(ctx context.Context, tx repository.TxInterface, issue uuid.UUID) (bool, error) {
	return self.base.Client(tx).GithubIssueLink.Query().Where(githubissuelink.ID(issue)).Exist(ctx)
}
