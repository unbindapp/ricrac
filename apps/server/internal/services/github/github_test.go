package github_service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"ricrac/config"
	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/integrations/github"
	"ricrac/internal/services"
	mocks_services_access "ricrac/mocks/services/access"
	mocks_services_attachment "ricrac/mocks/services/attachment"
	mocks_services_comment "ricrac/mocks/services/comment"
	mocks_services_events "ricrac/mocks/services/events"
	mocks_services_issue "ricrac/mocks/services/issue"
	mocks_services_resolve "ricrac/mocks/services/resolve"
)

type GitHubServiceSuite struct {
	services.ServiceTestSuite
	issues  *mocks_services_issue.IssueServiceMock
	service *GitHubService
	actor   *auth.Actor
}

func (suite *GitHubServiceSuite) SetupTest() {
	suite.ServiceTestSuite.SetupTest()
	cfg := config.Config{PublicURL: "http://localhost:3000", SecretKey: "test-secret-key-that-is-long-enough-123"}
	suite.issues = mocks_services_issue.NewIssueServiceMock(suite.T())
	suite.service = NewGitHubService(cfg, auth.NewKeys(cfg), suite.MockRepo, mocks_services_access.NewAccessServiceMock(suite.T()),
		mocks_services_resolve.NewResolveServiceMock(suite.T()), mocks_services_events.NewEventsServiceMock(suite.T()), suite.issues,
		mocks_services_comment.NewCommentServiceMock(suite.T()), mocks_services_attachment.NewAttachmentServiceMock(suite.T()), nil)
	suite.service.SetApp(github.NewApp(github.Config{Slug: "ricrac", WebhookSecret: "whsec", WebURL: "https://github.com"}, nil))
	suite.actor = auth.NewActor(uuid.New(), uuid.New(), auth.Admin, auth.EveryAccess, nil,
		func(context.Context, *auth.Actor) ([]uuid.UUID, error) { return nil, nil })
}

func TestGitHubService(t *testing.T) {
	suite.Run(t, new(GitHubServiceSuite))
}

func (suite *GitHubServiceSuite) TestInstallStatesAreSignedAndExpire() {
	workspace, user := uuid.New(), uuid.New()
	sign := func(expires time.Time) string {
		return suite.service.keys.Sign(fmt.Sprintf("github-install:%d:%s:%s", expires.Unix(), workspace, user))
	}
	gotWorkspace, gotUser, expired, ok := suite.service.ReadInstallState(sign(time.Now().Add(time.Hour)))
	suite.True(ok)
	suite.False(expired)
	suite.Equal(workspace, gotWorkspace)
	suite.Equal(user, gotUser)

	gotWorkspace, _, expired, ok = suite.service.ReadInstallState(sign(time.Now().Add(-time.Minute)))
	suite.True(ok && expired, "an old state still names its workspace")
	suite.Equal(workspace, gotWorkspace)

	_, _, _, ok = suite.service.ReadInstallState(sign(time.Now().Add(time.Hour)) + "x")
	suite.False(ok)
	_, _, _, ok = suite.service.ReadInstallState(suite.service.keys.Sign("github-link:1:" + user.String() + ":/"))
	suite.False(ok)
}

func (suite *GitHubServiceSuite) TestLinksBelongToWhoeverStartedThem() {
	user := uuid.New()
	state := suite.service.keys.Sign(fmt.Sprintf("github-link:%d:%s:%s", time.Now().Add(time.Minute).Unix(), user, "/acme/settings"))
	got, returnTo, err := suite.service.readLink(Session{User: user, SignedIn: true}, state)
	suite.NoError(err)
	suite.Equal(user, got)
	suite.Equal("/acme/settings", returnTo)

	_, returnTo, err = suite.service.readLink(Session{User: uuid.New(), SignedIn: true}, state)
	suite.Error(err)
	suite.Equal("/acme/settings", returnTo, "someone else is sent back where the link was started")

	outside := suite.service.keys.Sign(fmt.Sprintf("github-link:%d:%s:%s", time.Now().Add(time.Minute).Unix(), user, "//evil.example"))
	_, returnTo, _ = suite.service.readLink(Session{User: user, SignedIn: true}, outside)
	suite.Equal("/", returnTo)
	suite.Equal("/a?b=1&github_error=failed", withGitHubError("/a?b=1", "failed"))
}

func (suite *GitHubServiceSuite) installations(logins ...string) {
	var rows []*ent.GithubInstallation
	for i, login := range logins {
		rows = append(rows, &ent.GithubInstallation{ID: int64(70 + i), AccountLogin: login})
	}
	suite.MockGitHubRepo.EXPECT().Installations(mock.Anything, suite.actor.Workspace).Return(rows, nil).Once()
}

func (suite *GitHubServiceSuite) TestDisconnectingNamesTheInstallationWhenThereAreSeveral() {
	suite.installations()
	_, err := suite.service.Disconnect(suite.Ctx, suite.actor, &DisconnectInput{})
	suite.Equal("GitHub isn't connected to this workspace", errdefs.As(err).Message)

	suite.installations("acme-org", "acme-labs")
	_, err = suite.service.Disconnect(suite.Ctx, suite.actor, &DisconnectInput{})
	suite.Equal("GitHub is installed on several accounts; pass installation. Options: acme-org, acme-labs.", errdefs.As(err).Message)

	suite.installations("acme-org", "acme-labs")
	_, err = suite.service.Disconnect(suite.Ctx, suite.actor, &DisconnectInput{Installation: new("other")})
	suite.Equal(`GitHub isn't installed on "other" here. Options: acme-org, acme-labs.`, errdefs.As(err).Message)
}

func (suite *GitHubServiceSuite) TestInstallationsAreNamedByLoginOrID() {
	rows := []*ent.GithubInstallation{{ID: 70, AccountLogin: "acme-org"}, {ID: 71, AccountLogin: "acme-labs"}}
	chosen, err := chooseInstallation(rows, new(" ACME-labs "))
	suite.NoError(err)
	suite.Equal(int64(71), chosen.ID)
	chosen, err = chooseInstallation(rows, new("70"))
	suite.NoError(err)
	suite.Equal("acme-org", chosen.AccountLogin)
}

func (suite *GitHubServiceSuite) TestWebhooksNeedTheSecretsSignature() {
	body := []byte(`{"installation": {"id": 77}}`)
	err := suite.service.HandleWebhook(suite.Ctx, "ping", github.Sign("wrong", body), body)
	suite.Equal(401, errdefs.As(err).Status)
	err = suite.service.HandleWebhook(suite.Ctx, "ping", github.Sign("whsec", []byte("{")), []byte("{"))
	suite.Equal("The body isn't a GitHub event", errdefs.As(err).Message)
}

func (suite *GitHubServiceSuite) TestSyncsKeepTheirRepository() {
	_, err := suite.service.SaveRepoSync(suite.Ctx, suite.actor, &SaveRepoSyncInput{ID: new("x"), Repository: new("acme/other")})
	suite.Equal("A sync's repository can't change; delete it and sync the other repository", errdefs.As(err).Message)
}

func (suite *GitHubServiceSuite) TestEditsOnGitHubChangeOnlyWhatDiffers() {
	row := &ent.Issue{ID: uuid.New(), Title: "Logs stop", Description: "After ten minutes."}
	event := &github.IssuesEvent{Action: "edited", Issue: github.Issue{Title: "  Logs stop ", Body: "After ten minutes."}}
	input, _, err := suite.service.issueChange(suite.Ctx, suite.actor, row, event)
	suite.NoError(err)
	suite.Nil(input)

	event.Issue.Body = "After 10 minutes."
	input, label, err := suite.service.issueChange(suite.Ctx, suite.actor, row, event)
	suite.NoError(err)
	suite.Equal("Edited on GitHub", label)
	suite.Nil(input.Title)
	suite.Equal("After 10 minutes.", *input.Description)
}

func (suite *GitHubServiceSuite) TestClosingOnGitHubLeavesIssuesClosedTheSameWay() {
	row := &ent.Issue{ID: uuid.New(), StateID: uuid.New(), TeamID: uuid.New()}
	suite.issues.EXPECT().StateType(mock.Anything, mock.Anything, row.StateID).Return("canceled", nil)
	event := &github.IssuesEvent{Action: "closed", Issue: github.Issue{State: "closed", StateReason: "duplicate"}}
	input, _, err := suite.service.issueChange(suite.Ctx, suite.actor, row, event)
	suite.NoError(err)
	suite.Nil(input)
}

func (suite *GitHubServiceSuite) TestAssigningOnGitHubSkipsTheCurrentAssignee() {
	assignee := uuid.New()
	row := &ent.Issue{ID: uuid.New(), TeamID: uuid.New(), AssigneeID: &assignee}
	suite.MockGitHubRepo.EXPECT().MemberByLogin(mock.Anything, suite.actor.Workspace, "eve-gh").Return(assignee, true, nil)
	suite.MockGitHubRepo.EXPECT().CanSeeTeam(mock.Anything, row.TeamID, assignee).Return(true, nil)
	event := &github.IssuesEvent{Action: "assigned", Assignee: &github.Account{Login: "Eve-GH"}}
	input, _, err := suite.service.issueChange(suite.Ctx, suite.actor, row, event)
	suite.NoError(err)
	suite.Nil(input)
}

func TestRicracChangesBecomeAGitHubPatch(t *testing.T) {
	state := func(from, to string) M {
		return M{"state": M{"from": M{"type": from}, "to": M{"type": to}}}
	}
	cases := []struct {
		changes M
		want    M
	}{
		{M{"title": M{}}, M{"title": "T"}},
		{M{"description": M{}}, M{"body": "D"}},
		{state("started", "completed"), M{"state": "closed", "state_reason": "completed"}},
		{state("completed", "canceled"), M{"state": "closed", "state_reason": "not_planned"}},
		{state("canceled", "backlog"), M{"state": "open"}},
		{state("backlog", "started"), M{}},
	}
	for _, c := range cases {
		got := issuePatch(c.changes, "T", "D", nil)
		if fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("%v: got %v, want %v", c.changes, got, c.want)
		}
	}
	duplicateOf := int64(3500001347)
	got := issuePatch(state("backlog", "canceled"), "T", "D", &duplicateOf)
	want := M{"state": "closed", "state_reason": "duplicate", "duplicate_issue_id": duplicateOf}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("duplicate: got %v, want %v", got, want)
	}
	got = issuePatch(state("backlog", "completed"), "T", "D", &duplicateOf)
	if got["state_reason"] != "completed" {
		t.Errorf("a completed duplicate stays completed, got %v", got)
	}
}

func TestLabelChangesAreAddedAndRemovedNames(t *testing.T) {
	added, removed := labelChanges([]any{"Bug", "ui"}, []any{"ui", "Feature"})
	if fmt.Sprint(added) != "[Feature]" || fmt.Sprint(removed) != "[Bug]" {
		t.Errorf("added %v, removed %v", added, removed)
	}
}

func TestGitHubTitlesAndAuthors(t *testing.T) {
	if clampTitle("  ") != "Untitled GitHub issue" {
		t.Error("an empty title gets a placeholder")
	}
	if got := clampTitle(strings.Repeat("é", 300)); len([]rune(got)) != 255 {
		t.Errorf("titles are cut at 255 characters, got %d", len([]rune(got)))
	}
	if byLine("Closed on GitHub", uuid.Nil, "octocat") != "Closed on GitHub by @octocat" {
		t.Error("people without an account are named")
	}
	if byLine("Closed on GitHub", uuid.New(), "octocat") != "Closed on GitHub" {
		t.Error("people with an account are the actor")
	}
}
