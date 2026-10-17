package statusupdate_service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/services"
	mocks_services_events "ricrac/mocks/services/events"
	mocks_services_resolve "ricrac/mocks/services/resolve"
)

type StatusUpdateServiceSuite struct {
	services.ServiceTestSuite
	service *StatusUpdateService
	actor   *auth.Actor
}

func (suite *StatusUpdateServiceSuite) SetupTest() {
	suite.ServiceTestSuite.SetupTest()
	suite.service = NewStatusUpdateService(suite.MockRepo, mocks_services_resolve.NewResolveServiceMock(suite.T()), mocks_services_events.NewEventsServiceMock(suite.T()))
	suite.actor = auth.NewActor(uuid.New(), uuid.New(), auth.Member, auth.EveryAccess, nil, nil)
}

func TestStatusUpdateService(t *testing.T) {
	suite.Run(t, new(StatusUpdateServiceSuite))
}

func (suite *StatusUpdateServiceSuite) TestUpdatesAreOnOneProjectOrInitiative() {
	_, err := suite.service.Save(suite.Ctx, suite.actor, &SaveInput{Body: new("Going well")})
	suite.Equal("Name the project or initiative the update is about", errdefs.As(err).Message)
	_, err = suite.service.Save(suite.Ctx, suite.actor, &SaveInput{Project: new("Launch"), Initiative: new("Growth")})
	suite.Equal("Pass either project or initiative, not both", errdefs.As(err).Message)
	_, err = suite.service.Save(suite.Ctx, suite.actor, &SaveInput{Project: new("Launch"), Type: new("initiative")})
	suite.Equal(`type "initiative" doesn't match; it is project with project, initiative with initiative`, errdefs.As(err).Message)
}

func (suite *StatusUpdateServiceSuite) TestOnlyTheAuthorEditsAnUpdate() {
	_, err := suite.service.Save(suite.Ctx, suite.actor, &SaveInput{ID: new(uuid.NewString()), Project: new("Launch")})
	suite.Equal("An update stays on its project or initiative; pass only body and health", errdefs.As(err).Message)

	id := uuid.New()
	project := uuid.New()
	author := uuid.New()
	suite.MockStatusUpdateRepo.EXPECT().Get(suite.Ctx, suite.actor.Workspace, id).
		Return(&ent.StatusUpdate{ID: id, ProjectID: &project, UserID: &author}, nil)
	resolve := mocks_services_resolve.NewResolveServiceMock(suite.T())
	resolve.EXPECT().Project(suite.Ctx, suite.actor, project.String()).Return(project, nil)
	suite.MockProjectRepo.EXPECT().Get(suite.Ctx, nil, project).Return(&ent.Project{ID: project, Name: "Launch"}, nil)
	suite.service.resolve = resolve
	_, err = suite.service.Save(suite.Ctx, suite.actor, &SaveInput{ID: new(id.String()), Body: new("Edited")})
	suite.Equal("Only the author can edit a status update", errdefs.As(err).Message)
}
