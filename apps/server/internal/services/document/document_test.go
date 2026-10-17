package document_service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"ricrac/config"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/services"
	mocks_services_events "ricrac/mocks/services/events"
	mocks_services_resolve "ricrac/mocks/services/resolve"
)

type DocumentServiceSuite struct {
	services.ServiceTestSuite
	resolve *mocks_services_resolve.ResolveServiceMock
	service *DocumentService
	actor   *auth.Actor
}

func (suite *DocumentServiceSuite) SetupTest() {
	suite.ServiceTestSuite.SetupTest()
	suite.resolve = mocks_services_resolve.NewResolveServiceMock(suite.T())
	suite.service = NewDocumentService(config.Config{}, suite.MockRepo, suite.resolve, mocks_services_events.NewEventsServiceMock(suite.T()))
	suite.actor = auth.NewActor(uuid.New(), uuid.New(), auth.Member, auth.EveryAccess, nil,
		func(context.Context, *auth.Actor) ([]uuid.UUID, error) { return nil, nil })
	suite.ExpectTx()
}

func TestDocumentService(t *testing.T) {
	suite.Run(t, new(DocumentServiceSuite))
}

func (suite *DocumentServiceSuite) TestADocumentBelongsToOneParent() {
	suite.resolve.EXPECT().OptionalTeam(mock.Anything, suite.actor, (*string)(nil)).Return(nil, nil)
	_, err := suite.service.Save(suite.Ctx, suite.actor, &SaveInput{Title: new("Plan"), Project: new("Launch"), Issue: new("ENG-1")})
	suite.Equal("A document belongs to one of project, initiative, team, issue, cycle or release; pass only one", errdefs.As(err).Message)

	_, err = suite.service.Save(suite.Ctx, suite.actor, &SaveInput{Title: new("Plan")})
	suite.Equal("Say where the document belongs: one of project, initiative, team, issue, cycle or release", errdefs.As(err).Message)
}

func (suite *DocumentServiceSuite) TestTeamWithCycleNamesTheCyclesTeam() {
	team, cycle := uuid.New(), uuid.New()
	suite.resolve.EXPECT().OptionalTeam(mock.Anything, suite.actor, mock.Anything).Return(&team, nil)
	suite.resolve.EXPECT().Cycle(mock.Anything, suite.actor, &team, "current").Return(cycle, nil)
	parent, err := suite.service.parentOf(suite.Ctx, suite.actor, &SaveInput{Team: new("ENG"), Cycle: new("current")})
	suite.NoError(err)
	suite.Equal(&cycle, parent.Cycle)
	suite.Nil(parent.Team)
}

func (suite *DocumentServiceSuite) TestPatchesAndRestoresOnlyUpdate() {
	team := uuid.New()
	suite.resolve.EXPECT().OptionalTeam(mock.Anything, suite.actor, mock.Anything).Return(&team, nil)
	_, err := suite.service.Save(suite.Ctx, suite.actor, &SaveInput{Team: new("ENG"), Patch: []args.TextEdit{{Op: "append", Text: "x"}}})
	suite.Equal("patch only applies when updating; pass content", errdefs.As(err).Message)
	_, err = suite.service.Save(suite.Ctx, suite.actor, &SaveInput{Team: new("ENG"), RestoreVersion: new(int64(3))})
	suite.Equal("restoreVersion only applies when updating", errdefs.As(err).Message)
}

func (suite *DocumentServiceSuite) TestABurstOfEditsByOnePersonKeepsOneVersion() {
	id := uuid.New()
	other := uuid.New()
	suite.MockDocumentRepo.EXPECT().HasRecentVersion(mock.Anything, mock.Anything, id).Return(true, nil)
	suite.NoError(suite.service.keepVersion(suite.Ctx, nil, suite.actor, id, "Plan", "old", &suite.actor.User, false))

	suite.MockDocumentRepo.EXPECT().AddVersion(mock.Anything, mock.Anything, id, "Plan", "old", &other).Return(nil).Once()
	suite.NoError(suite.service.keepVersion(suite.Ctx, nil, suite.actor, id, "Plan", "old", &other, false))

	suite.MockDocumentRepo.EXPECT().AddVersion(mock.Anything, mock.Anything, id, "Plan", "old", &suite.actor.User).Return(nil).Once()
	suite.NoError(suite.service.keepVersion(suite.Ctx, nil, suite.actor, id, "Plan", "old", &suite.actor.User, true))
}
