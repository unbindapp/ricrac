package favorite_service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/services"
	mocks_services_resolve "ricrac/mocks/services/resolve"
)

type FavoriteServiceSuite struct {
	services.ServiceTestSuite
	resolve *mocks_services_resolve.ResolveServiceMock
	service *FavoriteService
	actor   *auth.Actor
}

func (suite *FavoriteServiceSuite) SetupTest() {
	suite.ServiceTestSuite.SetupTest()
	suite.resolve = mocks_services_resolve.NewResolveServiceMock(suite.T())
	suite.service = NewFavoriteService(suite.MockRepo, suite.resolve)
	suite.actor = auth.NewActor(uuid.New(), uuid.New(), auth.Member, auth.EveryAccess, nil,
		func(context.Context, *auth.Actor) ([]uuid.UUID, error) { return nil, nil })
}

func TestFavoriteService(t *testing.T) {
	suite.Run(t, new(FavoriteServiceSuite))
}

func (suite *FavoriteServiceSuite) TestTypesAreTheAPIsNames() {
	stored, err := storedType(" CustomView ")
	suite.NoError(err)
	suite.Equal("custom_view", stored)
	_, err = storedType("custom_view")
	suite.Equal(`type "custom_view" isn't one of issue, project, initiative, cycle, customView, document, label, team, user, release`,
		errdefs.As(err).Message)
}

func (suite *FavoriteServiceSuite) TestRemovingByIDFallsBackToTheTarget() {
	issue := uuid.New()
	favorite := uuid.New()
	suite.resolve.EXPECT().OptionalTeam(mock.Anything, suite.actor, (*string)(nil)).Return(nil, nil)
	suite.resolve.EXPECT().Issue(mock.Anything, suite.actor, issue.String()).Return(issue, nil)
	suite.MockFavoriteRepo.EXPECT().RemoveTarget(mock.Anything, suite.actor.User, suite.actor.Workspace, "issue", issue).Return(favorite, true, nil)
	removed, err := suite.service.Remove(suite.Ctx, suite.actor, &RemoveInput{ID: new(issue.String()), Type: new("issue")})
	suite.NoError(err)
	suite.Equal(favorite.String(), removed.ID)

	suite.MockFavoriteRepo.EXPECT().Remove(mock.Anything, issue, suite.actor.User, suite.actor.Workspace).Return(false, nil)
	_, err = suite.service.Remove(suite.Ctx, suite.actor, &RemoveInput{ID: new(issue.String())})
	suite.Equal("No such favorite; pass its id, or type and target", errdefs.As(err).Message)
}
