package label_service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	resolve_repo "ricrac/internal/repositories/resolve"
	"ricrac/internal/services"
	mocks_services_events "ricrac/mocks/services/events"
	mocks_services_resolve "ricrac/mocks/services/resolve"
)

type LabelServiceSuite struct {
	services.ServiceTestSuite
	resolve *mocks_services_resolve.ResolveServiceMock
	events  *mocks_services_events.EventsServiceMock
	service *LabelService
	actor   *auth.Actor
	team    uuid.UUID
}

func (suite *LabelServiceSuite) SetupTest() {
	suite.ServiceTestSuite.SetupTest()
	suite.resolve = mocks_services_resolve.NewResolveServiceMock(suite.T())
	suite.events = mocks_services_events.NewEventsServiceMock(suite.T())
	suite.service = NewLabelService(suite.MockRepo, suite.resolve, suite.events)
	suite.team = uuid.New()
	suite.actor = auth.NewActor(uuid.New(), uuid.New(), auth.Member, auth.EveryAccess, nil,
		func(context.Context, *auth.Actor) ([]uuid.UUID, error) { return []uuid.UUID{suite.team}, nil })
	suite.ExpectTx()
}

func TestLabelService(t *testing.T) {
	suite.Run(t, new(LabelServiceSuite))
}

func (suite *LabelServiceSuite) labels(rows ...resolve_repo.LabelRow) {
	suite.MockResolveRepo.EXPECT().AllIssueLabels(mock.Anything, suite.actor.Workspace, []uuid.UUID{suite.team}).Return(rows, nil)
}

func (suite *LabelServiceSuite) TestGuestsCantChangeLabels() {
	guest := auth.NewActor(uuid.New(), suite.actor.Workspace, auth.Guest, auth.EveryAccess, nil, nil)
	_, err := suite.service.Save(suite.Ctx, guest, &SaveInput{Name: new("Bug")})
	suite.Equal("Guests can't do this", errdefs.As(err).Message)
	_, err = suite.service.Delete(suite.Ctx, guest, &LabelInput{ID: "Bug"})
	suite.Equal(403, errdefs.As(err).Status)
}

func (suite *LabelServiceSuite) TestGroupedLabelsAreNamedWithTheirGroup() {
	group := "Type"
	bug := resolve_repo.LabelRow{ID: uuid.New(), Name: "Bug", Parent: &group}
	other := resolve_repo.LabelRow{ID: uuid.New(), Name: "Bug"}
	suite.labels(bug, other)
	found, err := suite.service.find(suite.Ctx, suite.actor, "type/bug", nil)
	suite.NoError(err)
	suite.Equal(bug.ID, found)

	suite.labels(bug, other)
	_, err = suite.service.find(suite.Ctx, suite.actor, "Bug", nil)
	suite.Equal(`"Bug" matches several labels; pass team, name it as Group/Bug, or use its id`, errdefs.As(err).Message)
}

func (suite *LabelServiceSuite) TestUnknownLabelsListTheOptions() {
	suite.labels(resolve_repo.LabelRow{ID: uuid.New(), Name: "Feature"})
	_, err := suite.service.find(suite.Ctx, suite.actor, "Bug", nil)
	suite.Equal(`No label "Bug". Options: Feature.`, errdefs.As(err).Message)
}

func (suite *LabelServiceSuite) TestNamesAreUniqueInTheirPlace() {
	label := uuid.New()
	suite.MockLabelRepo.EXPECT().GetForUpdate(mock.Anything, mock.Anything, label).
		Return(&ent.IssueLabel{ID: label, Name: "Bug", TeamID: &suite.team, GroupType: "singleSelect"}, nil)
	suite.MockLabelRepo.EXPECT().NameTaken(mock.Anything, mock.Anything, suite.actor.Workspace, &suite.team, (*uuid.UUID)(nil), "Defect", label).Return(true, nil)
	err := suite.service.apply(suite.Ctx, nil, suite.actor, label, &SaveInput{Name: new("Defect")})
	suite.Equal(`There already is a label named "Defect" in this team`, errdefs.As(err).Message)
}

func (suite *LabelServiceSuite) TestGroupsOnIssuesCantBeMade() {
	label := uuid.New()
	suite.MockLabelRepo.EXPECT().GetForUpdate(mock.Anything, mock.Anything, label).
		Return(&ent.IssueLabel{ID: label, Name: "Bug", GroupType: "singleSelect"}, nil)
	suite.MockLabelRepo.EXPECT().OnIssues(mock.Anything, mock.Anything, label).Return(true, nil)
	err := suite.service.apply(suite.Ctx, nil, suite.actor, label, &SaveInput{IsGroup: new(true)})
	suite.Equal("Bug is on issues, and a group can't be; create a new group instead", errdefs.As(err).Message)
}
