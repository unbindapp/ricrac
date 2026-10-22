package project_service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"ricrac/config"
	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	project_repo "ricrac/internal/repositories/project"
	"ricrac/internal/services"
	mocks_services_events "ricrac/mocks/services/events"
	mocks_services_milestone "ricrac/mocks/services/milestone"
	mocks_services_resolve "ricrac/mocks/services/resolve"
	mocks_services_statusupdate "ricrac/mocks/services/statusupdate"
)

type ProjectServiceSuite struct {
	services.ServiceTestSuite
	resolve *mocks_services_resolve.ResolveServiceMock
	service *ProjectService
	actor   *auth.Actor
}

func (suite *ProjectServiceSuite) SetupTest() {
	suite.ServiceTestSuite.SetupTest()
	suite.resolve = mocks_services_resolve.NewResolveServiceMock(suite.T())
	suite.service = NewProjectService(config.Config{}, suite.MockRepo, suite.resolve, mocks_services_events.NewEventsServiceMock(suite.T()),
		mocks_services_milestone.NewMilestoneServiceMock(suite.T()), mocks_services_statusupdate.NewStatusUpdateServiceMock(suite.T()), nil)
	suite.actor = auth.NewActor(uuid.New(), uuid.New(), auth.Member, auth.EveryAccess, nil,
		func(context.Context, *auth.Actor) ([]uuid.UUID, error) { return nil, nil })
	suite.ExpectTx()
}

func TestProjectService(t *testing.T) {
	suite.Run(t, new(ProjectServiceSuite))
}

func (suite *ProjectServiceSuite) labels(rows ...project_repo.LabelRow) {
	suite.MockProjectRepo.EXPECT().AllLabels(mock.Anything, suite.actor.Workspace).Return(rows, nil)
}

func (suite *ProjectServiceSuite) TestLabelsInGroupsAreNamedWithTheirGroup() {
	team, area := "Team", "Area"
	inTeam := project_repo.LabelRow{ID: uuid.New(), Name: "Platform", Parent: &team}
	inArea := project_repo.LabelRow{ID: uuid.New(), Name: "Platform", Parent: &area}
	suite.labels(inTeam, inArea)
	found, err := suite.service.labelID(suite.Ctx, suite.actor, "area/platform")
	suite.NoError(err)
	suite.Equal(inArea.ID, found)

	suite.labels(inTeam, inArea)
	_, err = suite.service.labelID(suite.Ctx, suite.actor, "Platform")
	suite.Equal(`"Platform" matches labels in several groups; name it as Group/Platform`, errdefs.As(err).Message)
}

func (suite *ProjectServiceSuite) TestUnknownLabelsListTheOptions() {
	group := "Team"
	suite.labels(project_repo.LabelRow{ID: uuid.New(), Name: "Platform", Parent: &group}, project_repo.LabelRow{ID: uuid.New(), Name: "Q3"})
	_, err := suite.service.labelID(suite.Ctx, suite.actor, "Q4")
	suite.Equal(`No project label "Q4". Options: Team/Platform, Q3.`, errdefs.As(err).Message)
}

func (suite *ProjectServiceSuite) TestGuestsCantCreateProjects() {
	guest := auth.NewActor(uuid.New(), suite.actor.Workspace, auth.Guest, auth.EveryAccess, nil, nil)
	_, err := suite.service.Save(suite.Ctx, guest, &SaveInput{Name: new("Launch"), AddTeams: []string{"ENG"}})
	suite.Equal(403, errdefs.As(err).Status)
}

func (suite *ProjectServiceSuite) TestTeamsAreSetOrChangedNotBoth() {
	err := suite.service.teams(suite.Ctx, nil, suite.actor, uuid.New(), []string{"ENG"}, []string{"OPS"}, nil)
	suite.Equal("Pass either setTeams or addTeams/removeTeams, not both", errdefs.As(err).Message)
}

func (suite *ProjectServiceSuite) TestGuestsCantChangeInitiatives() {
	guest := auth.NewActor(uuid.New(), suite.actor.Workspace, auth.Guest, auth.EveryAccess, nil, nil)
	err := suite.service.initiatives(suite.Ctx, nil, guest, uuid.New(), nil, []string{"Growth"}, nil)
	suite.Equal("Guests can't change initiatives", errdefs.As(err).Message)
}

func (suite *ProjectServiceSuite) TestStampsFollowTheStatusType() {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	started := now.Add(-48 * time.Hour)
	project := &ent.Project{StartedAt: &started}

	suite.Equal(project_repo.Stamps{StartedAt: &started, CompletedAt: &now}, stamps(project, "completed", now))
	suite.Equal(project_repo.Stamps{StartedAt: &started}, stamps(project, "paused", now))
	suite.Equal(project_repo.Stamps{}, stamps(project, "backlog", now))
	suite.Equal(project_repo.Stamps{StartedAt: &now}, stamps(&ent.Project{}, "started", now))
}

func (suite *ProjectServiceSuite) TestTrashedThingsAreFoundByTheirSlugTail() {
	suite.Equal("0a1b2c3d4e5f", TrashedSlugTail("https://ricrac.app/acme/project/launch-v2-0a1b2c3d4e5f?tab=issues"))
	suite.Equal("0a1b2c3d4e5f", TrashedSlugTail("acme/project/launch-0a1b2c3d4e5f/"))
	suite.Equal("launch-v2", TrashedSlugTail("launch-v2"))
}
