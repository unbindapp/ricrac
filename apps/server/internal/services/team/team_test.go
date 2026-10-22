package team_service

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
	team_repo "ricrac/internal/repositories/team"
	"ricrac/internal/services"
	mocks_services_events "ricrac/mocks/services/events"
	mocks_services_issue "ricrac/mocks/services/issue"
	mocks_services_resolve "ricrac/mocks/services/resolve"
	mocks_services_seed "ricrac/mocks/services/seed"
	mocks_services_workspace "ricrac/mocks/services/workspace"
)

type TeamServiceSuite struct {
	services.ServiceTestSuite
	resolve *mocks_services_resolve.ResolveServiceMock
	service *TeamService
	actor   *auth.Actor
	team    uuid.UUID
}

func (suite *TeamServiceSuite) SetupTest() {
	suite.ServiceTestSuite.SetupTest()
	suite.resolve = mocks_services_resolve.NewResolveServiceMock(suite.T())
	suite.service = NewTeamService(config.Config{}, suite.MockRepo, suite.resolve,
		mocks_services_events.NewEventsServiceMock(suite.T()), mocks_services_issue.NewIssueServiceMock(suite.T()),
		mocks_services_seed.NewSeedServiceMock(suite.T()), mocks_services_workspace.NewWorkspaceServiceMock(suite.T()))
	suite.team = uuid.New()
	suite.actor = auth.NewActor(uuid.New(), uuid.New(), auth.Member, auth.EveryAccess, nil,
		func(context.Context, *auth.Actor) ([]uuid.UUID, error) { return []uuid.UUID{suite.team}, nil })
	suite.ExpectTx()
}

func TestTeamService(t *testing.T) {
	suite.Run(t, new(TeamServiceSuite))
}

func (suite *TeamServiceSuite) TestOnlyMembersChangeAPrivateTeam() {
	suite.MockTeamRepo.EXPECT().Membership(mock.Anything, mock.Anything, suite.team, suite.actor.User).
		Return(team_repo.Membership{Private: true}, nil)
	err := suite.service.RequireCanEdit(suite.Ctx, nil, suite.actor, suite.team)
	suite.Equal("Only members of this private team can change it", errdefs.As(err).Message)

	admin := auth.NewActor(uuid.New(), suite.actor.Workspace, auth.Admin, auth.EveryAccess, nil, nil)
	suite.NoError(suite.service.RequireCanEdit(suite.Ctx, nil, admin, suite.team))
}

func (suite *TeamServiceSuite) TestTheDefaultEstimateStaysOnTheScale() {
	estimate := 3.0
	suite.MockTeamRepo.EXPECT().Get(mock.Anything, mock.Anything, suite.team).
		Return(&ent.Team{ID: suite.team, EstimateType: "exponential", DefaultEstimate: &estimate}, nil)
	suite.MockTeamRepo.EXPECT().MemberCounts(mock.Anything, mock.Anything, []uuid.UUID{suite.team}).
		Return(map[uuid.UUID]int{suite.team: 1}, nil)
	err := suite.service.check(suite.Ctx, nil, suite.team)
	suite.Equal("defaultEstimate 3 isn't on the exponential scale; use one of 1, 2, 4, 8, 16", errdefs.As(err).Message)
}

func (suite *TeamServiceSuite) TestTheLastMemberCantLeaveAPrivateTeam() {
	suite.resolve.EXPECT().TeamOrDefault(mock.Anything, suite.actor, new("ENG")).Return(suite.team, nil)
	suite.MockTeamRepo.EXPECT().Lock(mock.Anything, mock.Anything, suite.team).Return(nil)
	suite.MockTeamRepo.EXPECT().Unarchived(mock.Anything, mock.Anything, suite.actor.Workspace).
		Return([]*ent.Team{{ID: uuid.New()}, {ID: suite.team}}, nil)
	suite.MockTeamRepo.EXPECT().Membership(mock.Anything, mock.Anything, suite.team, suite.actor.User).
		Return(team_repo.Membership{Private: true, Member: true}, nil)
	suite.MockTeamRepo.EXPECT().RemoveMembers(mock.Anything, mock.Anything, suite.team, []uuid.UUID{suite.actor.User}).Return(nil)
	suite.MockTeamRepo.EXPECT().DropFromTriage(mock.Anything, mock.Anything, suite.team, []uuid.UUID{suite.actor.User}).Return(nil)
	suite.MockTeamRepo.EXPECT().Get(mock.Anything, mock.Anything, suite.team).Return(&ent.Team{ID: suite.team, Private: true}, nil)
	suite.MockTeamRepo.EXPECT().MemberCounts(mock.Anything, mock.Anything, []uuid.UUID{suite.team}).Return(map[uuid.UUID]int{}, nil)
	_, err := suite.service.Leave(suite.Ctx, suite.actor, &IDInput{ID: new("ENG")})
	suite.Equal("You're the last member of this private team; add someone else or delete the team", errdefs.As(err).Message)
}

func (suite *TeamServiceSuite) TestRotationsTakeTurns() {
	ann, bob := uuid.New(), uuid.New()
	start := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	today := start.AddDate(0, 0, 15)
	found := shifts([]uuid.UUID{ann, bob}, 1, start, today, 3, nil)
	suite.Equal([]uuid.UUID{ann, bob, ann}, []uuid.UUID{found[0].person, found[1].person, found[2].person})
	suite.Equal(start.AddDate(0, 0, 14), found[0].starts)

	mine := shifts([]uuid.UUID{ann, bob}, 2, start, today, 2, &bob)
	suite.Equal(start.AddDate(0, 0, 14), mine[0].starts)
	suite.Equal(start.AddDate(0, 0, 28), mine[0].ends)
	suite.Empty(shifts([]uuid.UUID{ann}, 1, start, today, 2, &bob))
}
