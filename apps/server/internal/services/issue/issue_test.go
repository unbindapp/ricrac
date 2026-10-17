package issue_service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"ricrac/config"
	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/services"
	mocks_services_events "ricrac/mocks/services/events"
	mocks_services_resolve "ricrac/mocks/services/resolve"
)

type IssueServiceSuite struct {
	services.ServiceTestSuite
	resolve *mocks_services_resolve.ResolveServiceMock
	events  *mocks_services_events.EventsServiceMock
	service *IssueService
	actor   *auth.Actor
	teams   []uuid.UUID
}

func (suite *IssueServiceSuite) SetupTest() {
	suite.ServiceTestSuite.SetupTest()
	suite.resolve = mocks_services_resolve.NewResolveServiceMock(suite.T())
	suite.events = mocks_services_events.NewEventsServiceMock(suite.T())
	suite.service = NewIssueService(config.Config{}, suite.MockRepo, suite.resolve, suite.events)
	suite.teams = []uuid.UUID{uuid.New(), uuid.New()}
	suite.actor = auth.NewActor(uuid.New(), uuid.New(), auth.Admin, auth.EveryAccess, nil,
		func(context.Context, *auth.Actor) ([]uuid.UUID, error) { return suite.teams, nil })
}

func TestIssueService(t *testing.T) {
	suite.Run(t, new(IssueServiceSuite))
}

func (suite *IssueServiceSuite) message(err error) string {
	suite.Require().Error(err)
	return errdefs.As(err).Message
}

func (suite *IssueServiceSuite) TestBulkChangesAreChecked() {
	_, err := suite.service.UpdateMany(suite.Ctx, suite.actor, &UpdateManyInput{Changes: M{"state": "Done"}})
	suite.Equal("Pass 1 to 100 issue ids", suite.message(err))
	_, err = suite.service.UpdateMany(suite.Ctx, suite.actor, &UpdateManyInput{IDs: []string{"ENG-1"}, Changes: M{"titel": "x"}})
	suite.Equal("Invalid changes: unknown field `titel`", suite.message(err))
	_, err = suite.service.UpdateMany(suite.Ctx, suite.actor, &UpdateManyInput{IDs: []string{"ENG-1"}, Changes: M{"id": "ENG-2"}})
	suite.Equal("changes can't have an id or a template", suite.message(err))
}

func (suite *IssueServiceSuite) TestNewIssuesNeedATeamWhenThereAreSeveral() {
	suite.MockIssueRepo.EXPECT().TeamsByKey(mock.Anything, suite.teams).
		Return([]*ent.Team{{ID: suite.teams[0], Key: "ENG"}, {ID: suite.teams[1], Key: "OPS"}}, nil).Once()
	suite.MockIssueRepo.EXPECT().TeamsByKey(mock.Anything, suite.teams).Return(nil, nil).Once()
	_, err := suite.service.onlyTeam(suite.Ctx, suite.actor)
	suite.Equal("Name the team: ENG, OPS", suite.message(err))
	_, err = suite.service.onlyTeam(suite.Ctx, suite.actor)
	suite.Equal("There are no teams yet; create one with save_team", suite.message(err))
}

func (suite *IssueServiceSuite) TestLabelsAreReplacedOrAddedNotBoth() {
	err := suite.service.applyLabels(suite.Ctx, nil, suite.actor, &ent.Issue{}, SaveInput{Labels: []string{"Bug"}, AddLabels: []string{"UI"}}, M{})
	suite.Equal("Pass either labels or addLabels/removeLabels, not both", suite.message(err))
}

func (suite *IssueServiceSuite) TestImportedIssuesKeepTheirNumbersAndTheRestFollow() {
	team := suite.teams[0]
	suite.MockIssueRepo.EXPECT().TakenNumbers(mock.Anything, mock.Anything, team, []int{7}).Return(nil, nil)
	suite.MockIssueRepo.EXPECT().Team(mock.Anything, mock.Anything, team).Return(&ent.Team{IssueCount: 3}, nil)
	numbers, err := suite.service.assignNumbers(suite.Ctx, nil, team, "ENG", []ImportIssue{{}, {Number: new(7)}, {}})
	suite.NoError(err)
	suite.Equal([]int{8, 7, 9}, numbers)
}

func (suite *IssueServiceSuite) TestImportedNumbersMustBeFree() {
	team := suite.teams[0]
	_, err := suite.service.assignNumbers(suite.Ctx, nil, team, "ENG", []ImportIssue{{Number: new(2)}, {Number: new(2)}})
	suite.Equal("ENG-2 appears twice in the import", suite.message(err))

	suite.MockIssueRepo.EXPECT().TakenNumbers(mock.Anything, mock.Anything, team, []int{2}).Return([]int{2}, nil)
	_, err = suite.service.assignNumbers(suite.Ctx, nil, team, "ENG", []ImportIssue{{Number: new(2)}})
	suite.Equal("This number is taken in the team: ENG-2", suite.message(err))
}
