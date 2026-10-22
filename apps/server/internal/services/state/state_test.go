package state_service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/services"
	mocks_services_events "ricrac/mocks/services/events"
	mocks_services_issue "ricrac/mocks/services/issue"
	mocks_services_resolve "ricrac/mocks/services/resolve"
	mocks_services_team "ricrac/mocks/services/team"
)

type StateServiceSuite struct {
	services.ServiceTestSuite
	issues  *mocks_services_issue.IssueServiceMock
	teams   *mocks_services_team.TeamServiceMock
	service *StateService
	actor   *auth.Actor
	team    uuid.UUID
}

func (suite *StateServiceSuite) SetupTest() {
	suite.ServiceTestSuite.SetupTest()
	suite.issues = mocks_services_issue.NewIssueServiceMock(suite.T())
	suite.teams = mocks_services_team.NewTeamServiceMock(suite.T())
	suite.service = NewStateService(suite.MockRepo, mocks_services_resolve.NewResolveServiceMock(suite.T()),
		mocks_services_events.NewEventsServiceMock(suite.T()), suite.issues, suite.teams)
	suite.team = uuid.New()
	suite.actor = auth.NewActor(uuid.New(), uuid.New(), auth.Member, auth.EveryAccess, nil,
		func(context.Context, *auth.Actor) ([]uuid.UUID, error) { return []uuid.UUID{suite.team}, nil })
	suite.ExpectTx()
}

func TestStateService(t *testing.T) {
	suite.Run(t, new(StateServiceSuite))
}

func (suite *StateServiceSuite) workflow(defaultState *uuid.UUID, kinds ...string) {
	var states []*ent.WorkflowState
	for _, kind := range kinds {
		states = append(states, &ent.WorkflowState{ID: uuid.New(), Type: kind})
	}
	suite.MockStateRepo.EXPECT().Active(mock.Anything, mock.Anything, suite.team).Return(states, nil).Once()
	suite.MockTeamRepo.EXPECT().Get(mock.Anything, mock.Anything, suite.team).
		Return(&ent.Team{ID: suite.team, DefaultIssueStateID: defaultState}, nil).Once()
}

func (suite *StateServiceSuite) TestATeamKeepsAnOpenACompletedAndACanceledStatus() {
	suite.workflow(nil, "backlog", "completed")
	err := suite.service.checkWorkflow(suite.Ctx, nil, suite.team)
	suite.Equal("A team needs at least one canceled status", errdefs.As(err).Message)

	suite.workflow(nil, "completed", "canceled")
	err = suite.service.checkWorkflow(suite.Ctx, nil, suite.team)
	suite.Equal("A team needs at least one status that isn't completed or canceled", errdefs.As(err).Message)
}

func (suite *StateServiceSuite) TestTheDefaultStatusStaysBacklogOrUnstarted() {
	started := uuid.New()
	suite.workflow(&started, "started", "completed", "canceled")
	suite.issues.EXPECT().StateType(mock.Anything, mock.Anything, started).Return("started", nil)
	err := suite.service.checkWorkflow(suite.Ctx, nil, suite.team)
	suite.Equal("The team's default status must stay a backlog or unstarted status", errdefs.As(err).Message)
}

func (suite *StateServiceSuite) TestDeletingAStatusWithIssuesNeedsATarget() {
	state := uuid.New()
	suite.MockStateRepo.EXPECT().Owned(mock.Anything, suite.actor.Workspace, state).Return(state, suite.team, true, nil)
	suite.teams.EXPECT().RequireCanEdit(mock.Anything, mock.Anything, suite.actor, suite.team).Return(nil)
	suite.MockStateRepo.EXPECT().GetForUpdate(mock.Anything, mock.Anything, state).Return(&ent.WorkflowState{
		ID: state, Name: "Review", Type: "started", Edges: ent.WorkflowStateEdges{Team: &ent.Team{ID: suite.team}},
	}, nil)
	suite.MockStateRepo.EXPECT().Issues(mock.Anything, mock.Anything, state).Return([]uuid.UUID{uuid.New(), uuid.New()}, nil)
	_, err := suite.service.Delete(suite.Ctx, suite.actor, &DeleteInput{ID: state.String()})
	suite.Equal("2 issues are in Review; pass moveIssuesTo (a status of the same team)", errdefs.As(err).Message)
}
